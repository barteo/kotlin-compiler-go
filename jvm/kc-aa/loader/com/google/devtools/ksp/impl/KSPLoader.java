package com.google.devtools.ksp.impl;

import com.google.devtools.ksp.processing.KSPConfig;
import java.io.ByteArrayInputStream;
import java.io.File;
import java.io.IOException;
import java.io.ObjectInputStream;
import java.lang.reflect.InvocationTargetException;
import java.net.URISyntaxException;
import java.net.URL;
import java.net.URLClassLoader;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.ServiceLoader;
import java.util.Set;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.FutureTask;
import java.util.stream.Stream;

/**
 * The class KSP's Gradle plugin calls to run KSP, for KSP on kc.
 *
 * <p>The plugin loads KSP's class path in one class loader, makes a child of it for the
 * processors, and calls {@code loadAndRunKSP} of the class with this name in the first. Stock
 * KSP's class path is safe to be a processor's parent, as its Analysis API jar has every
 * package it bundles renamed. kc's is not: kotlin-compiler bundles Guava, and the KSP fork
 * uses kotlinx.serialization, which a processor that brings its own version then gets half
 * of.
 *
 * <p>So this class stands alone in a jar, which goes on KSP's class path with what a processor
 * shares with KSP: KSP's API, its configuration classes, the Kotlin standard library and
 * kotlinx.coroutines. The KSP fork, kc's backend and their libraries are the jars of {@code
 * lib} next to this class's jar, in a class loader below this class's, which no processor
 * sees. The libkc the backend loads is {@code libkc.so} next to the jar, so a JVM's builds
 * all name one file.
 *
 * <p>KSP runs in a thread of its own, which ends with the run: kotlin-compiler leaves thread
 * locals behind, which keep the class loaders of a finished build for as long as the thread
 * lives, and the caller's thread is the build tool's to keep.
 *
 * <p>The plugin makes the processors' class loader anew for every task of every build, so each
 * run loads the processors' classes and runs them before the JVM has compiled them. With the
 * environment variable {@code KC_KSP_KEEP} set, the processors of a run come from a class
 * loader this class keeps for their class path, for as long as the jars stay the same, and
 * every task and build with that class path shares it. A processor then keeps its static
 * state from one run to the next, which is why the variable has to ask for it.
 *
 * <p>The system property {@code kc.ksp.stock:CLASSPATH}, where CLASSPATH is this class
 * loader's class path, gives the class path of stock KSP for this one. When it is set and
 * the run on kc fails, stock KSP runs in its place, over new class loaders for itself and for
 * the processors.
 */
public final class KSPLoader {
  /** The property's prefix; the paths of both class paths are joined by {@link File#pathSeparator}. */
  public static final String STOCK = "kc.ksp.stock:";

  private static final String NAME = KSPLoader.class.getName();
  private static final String PROVIDER = "com.google.devtools.ksp.processing.SymbolProcessorProvider";

  private static ClassLoader backend;

  /** A kept class loader of processors, with the sizes and times of its jars when it was made. */
  private record Kept(String jars, URLClassLoader loader) {}

  private static final Map<List<URL>, Kept> KEPT = new HashMap<>();

  private KSPLoader() {}

  public static int loadAndRunKSP(byte[] config, List<?> providers, int logLevel) throws Throwable {
    String module = config(config).getModuleName();
    long start = System.nanoTime();
    Object failure;
    try {
      ClassLoader context = Thread.currentThread().getContextClassLoader();
      List<?> own = System.getenv("KC_KSP_KEEP") == null ? providers : kept(providers);
      if (own != providers && context == providers.get(0).getClass().getClassLoader()) {
        context = own.get(0).getClass().getClassLoader();
      }
      FutureTask<Integer> run = new FutureTask<>(() -> run(backend(), config, own, logLevel));
      Thread thread = new Thread(run, "kc-ksp " + module);
      thread.setContextClassLoader(context);
      thread.start();
      int code;
      try {
        code = run.get();
      } catch (InterruptedException e) {
        thread.interrupt();
        throw e;
      } catch (ExecutionException e) {
        throw e.getCause();
      }
      if (code == 0) {
        System.out.println("kc: KSP on " + module + ": " + (System.nanoTime() - start) / 1_000_000 + " ms");
        return 0;
      }
      failure = code;
    } catch (InterruptedException e) {
      throw e;
    } catch (Throwable e) {
      failure = e;
    }
    String stock = System.getProperty(STOCK + classPath((URLClassLoader) KSPLoader.class.getClassLoader()));
    ClassLoader processors = providers.isEmpty() ? null : providers.get(0).getClass().getClassLoader();
    if (stock == null || !(processors instanceof URLClassLoader)) {
      if (failure instanceof Throwable) {
        throw (Throwable) failure;
      }
      return (Integer) failure;
    }
    System.out.println("kc: KSP on " + module + " failed, so stock KSP processes it: "
        + (failure instanceof Throwable ? failure : "exit code " + failure));
    return onStock(stock, (URLClassLoader) processors, config, providers, logLevel);
  }

  /**
   * The providers of the classes of {@code providers}, from the class loader kept for their class
   * path, or {@code providers} when their class loader isn't the plugin's.
   */
  private static synchronized List<?> kept(List<?> providers) throws Exception {
    ClassLoader given = providers.isEmpty() ? null : providers.get(0).getClass().getClassLoader();
    if (!(given instanceof URLClassLoader) || given.getParent() != KSPLoader.class.getClassLoader()) {
      return providers;
    }
    List<URL> urls = List.of(((URLClassLoader) given).getURLs());
    StringBuilder jars = new StringBuilder();
    for (URL url : urls) {
      File f = new File(url.toURI());
      jars.append(f.length()).append(' ').append(f.lastModified()).append(' ');
    }
    Kept kept = KEPT.get(urls);
    boolean made = kept == null || !kept.jars.equals(jars.toString());
    if (made) {
      kept = new Kept(jars.toString(), new URLClassLoader(urls.toArray(new URL[0]), given.getParent()));
      KEPT.put(urls, kept);
    }
    Map<String, Object> byName = new HashMap<>();
    for (Object p : ServiceLoader.load(kept.loader.loadClass(PROVIDER), kept.loader)) {
      byName.put(p.getClass().getName(), p);
    }
    List<Object> own = new ArrayList<>();
    for (Object p : providers) {
      Object q = byName.get(p.getClass().getName());
      if (q == null) {
        return providers;
      }
      own.add(q);
    }
    if (System.getenv("KC_TIMING") != null) {
      System.err.println("kc-jvm: processors " + (made ? "loaded" : "kept"));
    }
    return own;
  }

  /** Runs stock KSP as the plugin does, after removing what the failed run wrote. */
  private static int onStock(String stock, URLClassLoader processors, byte[] config, List<?> providers, int logLevel)
      throws Exception {
    KSPConfig c = config(config);
    delete(c.getOutputBaseDir());
    delete(c.getCachesDir());
    Set<String> names = new HashSet<>();
    for (Object p : providers) {
      names.add(p.getClass().getName());
    }
    try (URLClassLoader ksp = new URLClassLoader(urls(stock.split(File.pathSeparator)), ClassLoader.getPlatformClassLoader());
        URLClassLoader own = new URLClassLoader(processors.getURLs(), ksp)) {
      List<Object> stockProviders = new ArrayList<>();
      for (Object p : ServiceLoader.load(own.loadClass(PROVIDER), own)) {
        if (names.contains(p.getClass().getName())) {
          stockProviders.add(p);
        }
      }
      return run(ksp, config, stockProviders, logLevel);
    }
  }

  private static int run(ClassLoader ksp, byte[] config, List<?> providers, int logLevel) throws Exception {
    try {
      return (Integer) ksp.loadClass(NAME).getMethod("loadAndRunKSP", byte[].class, List.class, int.class)
          .invoke(null, config, providers, logLevel);
    } catch (InvocationTargetException e) {
      if (e.getCause() instanceof Error) {
        throw (Error) e.getCause();
      }
      throw e.getCause() instanceof Exception ? (Exception) e.getCause() : e;
    }
  }

  /** The class loader of the KSP fork and kc's backend, made once. */
  private static synchronized ClassLoader backend() throws IOException, URISyntaxException, ClassNotFoundException {
    if (backend == null) {
      File home = new File(KSPLoader.class.getProtectionDomain().getCodeSource().getLocation().toURI()).getParentFile();
      File[] jars = new File(home, "lib").listFiles((dir, name) -> name.endsWith(".jar"));
      if (jars == null || jars.length == 0) {
        throw new IOException("no jars in " + new File(home, "lib"));
      }
      Arrays.sort(jars);
      String[] paths = new String[jars.length];
      for (int i = 0; i < jars.length; i++) {
        paths[i] = jars[i].getPath();
      }
      ClassLoader loader = new Backend(urls(paths), KSPLoader.class.getClassLoader());
      // KcNative reads the property when it initializes. A string literal is one object in a
      // JVM, so class loaders with different homes don't read each other's.
      synchronized ("kc.lib") {
        System.setProperty("kc.lib", new File(home, "libkc.so").getPath());
        Class.forName("io.github.barteo.kc.KcNative", true, loader);
      }
      backend = loader;
    }
    return backend;
  }

  /** Loads the KSP fork's class of this class's name itself, and every other class parent first. */
  private static final class Backend extends URLClassLoader {
    Backend(URL[] urls, ClassLoader parent) {
      super(urls, parent);
    }

    @Override
    protected Class<?> loadClass(String name, boolean resolve) throws ClassNotFoundException {
      if (!name.equals(NAME) && !name.startsWith(NAME + "$")) {
        return super.loadClass(name, resolve);
      }
      synchronized (getClassLoadingLock(name)) {
        Class<?> c = findLoadedClass(name);
        if (c == null) {
          c = findClass(name);
        }
        if (resolve) {
          resolveClass(c);
        }
        return c;
      }
    }
  }

  private static KSPConfig config(byte[] config) throws IOException, ClassNotFoundException {
    try (ObjectInputStream in = new ObjectInputStream(new ByteArrayInputStream(config))) {
      return (KSPConfig) in.readObject();
    }
  }

  private static String classPath(URLClassLoader loader) throws URISyntaxException {
    List<String> paths = new ArrayList<>();
    for (URL url : loader.getURLs()) {
      paths.add(new File(url.toURI()).getPath());
    }
    return String.join(File.pathSeparator, paths);
  }

  private static URL[] urls(String[] paths) throws IOException {
    URL[] urls = new URL[paths.length];
    for (int i = 0; i < paths.length; i++) {
      urls[i] = new File(paths[i]).toURI().toURL();
    }
    return urls;
  }

  private static void delete(File dir) throws IOException {
    if (!dir.exists()) {
      return;
    }
    try (Stream<Path> files = Files.walk(dir.toPath())) {
      for (Path p : (Iterable<Path>) files.sorted(Comparator.reverseOrder())::iterator) {
        Files.delete(p);
      }
    }
  }
}

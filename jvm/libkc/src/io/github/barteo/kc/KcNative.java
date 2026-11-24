package io.github.barteo.kc;

import java.io.File;
import java.io.IOException;
import java.io.InputStream;
import java.lang.invoke.MethodHandle;
import java.lang.invoke.MethodHandles;
import java.lang.invoke.MethodType;
import java.lang.reflect.InvocationTargetException;
import java.net.URL;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.ProtectionDomain;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

/**
 * The entry point of libkc (cmd/libkc in kotlin-compiler-go).
 *
 * <p>The library is loaded from the path in the system property {@code kc.lib}, or else as
 * {@code kc} from {@code java.library.path}, once in a JVM: by {@link KcBridge}, which the
 * first class loader to initialize this class sets up and every other one shares. So this
 * class works in any number of class loaders, alive together or one after another. They must
 * name the same library file: a JVM holds one libkc. Initializing this class also checks that
 * the library's ABI version is the one this class was written for.
 */
public final class KcNative {
  /** The ABI version this class expects; internal/kcabi.Version on the Go side. */
  public static final String ABI_VERSION = "kc-aa/0";

  public static final int OP_VERSION = 1;
  public static final int OP_ECHO = 2;
  public static final int OP_PANIC = 3;

  private static final MethodHandle CALL;
  private static final Map<Long, Map.Entry<Integer, byte[]>> AT_UNLOAD;

  static {
    try {
      Class<?> bridge = bridge();
      MethodHandles.Lookup lookup = MethodHandles.publicLookup();
      CALL = lookup.findStatic(bridge, "call", MethodType.methodType(byte[].class, int.class, byte[].class, int[].class));
      ClassLoader loader = KcNative.class.getClassLoader();
      if (loader == null) {
        AT_UNLOAD = new ConcurrentHashMap<>();
      } else {
        @SuppressWarnings("unchecked")
        Map<Long, Map.Entry<Integer, byte[]>> calls = (Map<Long, Map.Entry<Integer, byte[]>>)
            lookup.findStatic(bridge, "track", MethodType.methodType(Map.class, Object.class)).invoke((Object) loader);
        AT_UNLOAD = calls;
      }
    } catch (InvocationTargetException e) {
      if (e.getCause() instanceof Error) {
        throw (Error) e.getCause();
      }
      throw (Error) new UnsatisfiedLinkError("can't set up libkc: " + e.getCause()).initCause(e.getCause());
    } catch (RuntimeException | Error e) {
      throw e;
    } catch (Throwable e) {
      throw (Error) new UnsatisfiedLinkError("can't set up libkc: " + e).initCause(e);
    }
    String v = new String(call(OP_VERSION, new byte[0]), StandardCharsets.UTF_8);
    if (!v.equals(ABI_VERSION)) {
      throw new UnsatisfiedLinkError("libkc has ABI " + v + ", expected " + ABI_VERSION);
    }
  }

  private KcNative() {}

  /**
   * Returns the JVM's {@link KcBridge}, which isn't this class loader's: the one whose thread
   * runs, or else a new one, which loads the library.
   */
  private static Class<?> bridge() throws IOException, ReflectiveOperationException {
    String path = System.getProperty("kc.lib");
    String id = "java.library.path";
    if (path != null) {
      File f = new File(path).getCanonicalFile();
      path = f.getPath();
      id = path + " " + f.length() + " " + f.lastModified();
    }
    // A string literal is one object in a JVM, whichever class loader's class names it.
    synchronized (KcBridge.NAME) {
      Class<?> bridge = running();
      if (bridge == null) {
        bridge = define();
        bridge.getMethod("load", String.class, String.class).invoke(null, path, id);
      }
      Object loaded = bridge.getMethod("library").invoke(null);
      if (!id.equals(loaded)) {
        throw new UnsatisfiedLinkError("this JVM has loaded libkc (" + loaded + "), and can't load another (" + id + ")");
      }
      return bridge;
    }
  }

  private static Class<?> running() throws ClassNotFoundException {
    ThreadGroup root = Thread.currentThread().getThreadGroup();
    while (root.getParent() != null) {
      root = root.getParent();
    }
    Thread[] threads = new Thread[root.activeCount() + 16];
    while (root.enumerate(threads) == threads.length) {
      threads = new Thread[threads.length * 2];
    }
    for (Thread t : threads) {
      if (t != null && t.getName().equals(KcBridge.NAME)) {
        return Class.forName(KcBridge.NAME, true, t.getContextClassLoader());
      }
    }
    return null;
  }

  /**
   * Defines {@link KcBridge} from this class loader's class file in a class loader that sees
   * the JDK's classes only and keeps nothing of this one. A class loader keeps the protection
   * domains of the classes that are calling when it is made, and with them their class loaders,
   * so {@link KcBridge#loader} makes it, as a class a {@link Definer} defines.
   */
  private static Class<?> define() throws IOException, ReflectiveOperationException {
    String file = KcBridge.NAME.replace('.', '/') + ".class";
    byte[] bytes;
    try (InputStream in = KcNative.class.getResourceAsStream("/" + file)) {
      bytes = in.readAllBytes();
    }
    Path dir = Files.createTempDirectory("libkc");
    Path cls = dir.resolve(file);
    try {
      Files.createDirectories(cls.getParent());
      Files.write(cls, bytes);
      Object loader = new Definer().define(bytes).getMethod("loader", URL[].class).invoke(null, (Object) new URL[] {dir.toUri().toURL()});
      return Class.forName(KcBridge.NAME, true, (ClassLoader) loader);
    } finally {
      for (Path p = cls; p.startsWith(dir); p = p.getParent()) {
        Files.deleteIfExists(p);
      }
    }
  }

  /** Defines {@link KcBridge} with a protection domain that names no class loader. */
  private static final class Definer extends ClassLoader {
    Definer() {
      super(null);
    }

    Class<?> define(byte[] bytes) {
      return defineClass(KcBridge.NAME, bytes, 0, bytes.length, new ProtectionDomain(null, null));
    }
  }

  /**
   * Runs operation {@code op} on {@code request} and returns its response.
   *
   * @throws KcException if the operation fails
   * @throws KcPanicException if kc panicked, which is a bug in kc
   */
  public static byte[] call(int op, byte[] request) {
    int[] status = new int[1];
    byte[] response;
    try {
      response = (byte[]) CALL.invokeExact(op, request, status);
    } catch (RuntimeException | Error e) {
      throw e;
    } catch (Throwable e) {
      throw new KcException(e.toString());
    }
    if (status[0] == KcBridge.OK) {
      return response;
    }
    String message = new String(response, StandardCharsets.UTF_8);
    throw status[0] == KcBridge.PANIC ? new KcPanicException(message) : new KcException(message);
  }

  /**
   * Has libkc run operation {@code op} on {@code request} once this class's loader is
   * unreachable, unless {@link #cancelAtUnload} of {@code handle} comes first. It is how a
   * handle that a class loader still holds when it is dropped gets released.
   */
  public static void atUnload(long handle, int op, byte[] request) {
    AT_UNLOAD.put(handle, Map.entry(op, request));
  }

  /** Cancels {@link #atUnload} of {@code handle}. */
  public static void cancelAtUnload(long handle) {
    AT_UNLOAD.remove(handle);
  }
}

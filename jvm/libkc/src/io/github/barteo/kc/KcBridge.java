package io.github.barteo.kc;

import java.lang.invoke.MethodHandle;
import java.lang.invoke.MethodHandles;
import java.lang.invoke.MethodType;
import java.net.URL;
import java.net.URLClassLoader;
import java.nio.charset.StandardCharsets;
import java.security.CodeSource;
import javax.management.MBeanServer;
import javax.management.MBeanServerFactory;
import javax.management.ObjectName;

/**
 * kc's calls into libkc, made through the JVM's one {@link KcNative}.
 *
 * <p>A JVM lets one class loader load a native library, but the KSP Gradle plugin loads
 * kc-aa in a class loader of its own for each KSP class path a daemon sees (two projects
 * on different KSP or kc builds), and drops them when its class-loader cache service
 * closes. So {@code KcNative}, which loads
 * libkc, is loaded once per JVM, in a class loader that every copy of this class can
 * reach and that outlives them:
 *
 * <ul>
 *   <li>the system class loader, if it has {@code KcNative} (kc-aa on a {@code java}
 *       command's class path);
 *   <li>else a class loader of its own, named "libkc", whose parent is the platform class
 *       loader. The first caller makes it from the jar (or directory) this class came
 *       from, and registers a {@link KcLibrary} from it in a private {@link MBeanServer}
 *       whose default domain is {@value #DOMAIN}. {@link MBeanServerFactory} keeps that
 *       server, and so the class loader and libkc, for the life of the JVM. Later callers,
 *       in other class loaders, find {@code KcNative} through it. System properties would
 *       not do: a Gradle daemon restores them after each build.
 * </ul>
 *
 * <p>Each copy of this class checks that the loaded libkc has the ABI that copy was
 * compiled for, so a daemon holding libkc that is handed a kc-aa of another ABI fails with
 * a message, not a crash. Exceptions thrown by another class loader's {@code KcNative} are
 * turned into this class loader's {@link KcException} and {@link KcPanicException}.
 */
public final class KcBridge {
  /** The default domain of the MBeanServer that holds the shared class loader. */
  public static final String DOMAIN = "io.github.barteo.kc";

  private static final String PKG = "io.github.barteo.kc.";
  private static final ObjectName NAME = objectName(DOMAIN + ":type=KcLibrary");

  /** The JVM's KcNative. */
  private static final Class<?> NATIVE;
  /** KcNative.call(int, byte[]): byte[]. */
  private static final MethodHandle CALL;
  /** The libkc that KcNative loaded, as {@link #library} put it then. Set by findNative. */
  private static String loaded;

  static {
    NATIVE = findNative();
    try {
      CALL = MethodHandles.publicLookup().findStatic(NATIVE, "call",
          MethodType.methodType(byte[].class, int.class, byte[].class));
    } catch (ReflectiveOperationException e) {
      throw (LinkageError) new LinkageError("libkc: no KcNative.call in " + where()).initCause(e);
    }
    // KcNative checked libkc's ABI against its own when it loaded libkc; this checks it
    // against this copy's, which may come from another kc-aa. KcNative's constants are
    // compile-time constants, so they are this copy's and don't load this class loader's
    // KcNative.
    String v = new String(call(KcNative.OP_VERSION, new byte[0]), StandardCharsets.UTF_8);
    if (!v.equals(KcNative.ABI_VERSION)) {
      throw new UnsatisfiedLinkError("libkc has ABI " + v + ", but this kc-aa expects "
          + KcNative.ABI_VERSION + ": this JVM loaded libkc for another kc-aa (" + where()
          + "), and a JVM loads a native library once; restart it (gradle --stop for a "
          + "Gradle daemon)");
    }
  }

  private KcBridge() {}

  /**
   * Runs operation {@code op} on {@code request} and returns its response.
   *
   * @throws KcException if the operation fails
   * @throws KcPanicException if kc panicked, which is a bug in kc
   */
  public static byte[] call(int op, byte[] request) {
    try {
      return (byte[]) CALL.invokeExact(op, request);
    } catch (KcException | Error e) {
      throw e;
    } catch (RuntimeException e) {
      throw translate(e);
    } catch (Throwable t) {
      throw new IllegalStateException("libkc: KcNative.call threw a checked exception", t);
    }
  }

  /** Where the JVM's KcNative is: its class loader, and libkc's path. */
  public static String where() {
    ClassLoader l = NATIVE == null ? null : NATIVE.getClassLoader();
    String loader = l == null ? "?" : l.getName() + "@" + Integer.toHexString(System.identityHashCode(l));
    return "class loader " + loader + ", " + loaded;
  }

  /** libkc's path as KcNative loads it: the kc.lib property or java.library.path. */
  static String library() {
    String path = System.getProperty("kc.lib");
    return path != null ? path : "kc on java.library.path";
  }

  /** The JVM's KcNative, loaded and libkc with it. */
  private static Class<?> findNative() {
    ClassLoader system = ClassLoader.getSystemClassLoader();
    if (system.getResource("io/github/barteo/kc/KcNative.class") != null) {
      loaded = library();
      return load("KcNative", system);
    }
    // MBeanServerFactory's methods lock its class, so this lock covers them too: two
    // class loaders starting at once make one shared class loader.
    synchronized (MBeanServerFactory.class) {
      for (MBeanServer s : MBeanServerFactory.findMBeanServer(null)) {
        if (DOMAIN.equals(s.getDefaultDomain())) {
          try {
            ClassLoader shared = s.getClassLoaderFor(NAME);
            loaded = (String) s.getAttribute(NAME, "Library");
            warnIfOther(loaded);
            return load("KcNative", shared);
          } catch (javax.management.JMException e) {
            throw (LinkageError) new LinkageError("libkc: bad registration in MBeanServer " + DOMAIN).initCause(e);
          }
        }
      }
      return share();
    }
  }

  /**
   * Makes the shared class loader, loads KcNative and libkc in it, and registers it.
   * Called with MBeanServerFactory's lock held.
   */
  private static Class<?> share() {
    CodeSource cs = KcBridge.class.getProtectionDomain().getCodeSource();
    URL jar = cs == null ? null : cs.getLocation();
    if (jar == null) throw new LinkageError("libkc: can't tell which jar " + KcBridge.class + " came from");
    try (URLClassLoader shared = new URLClassLoader("libkc", new URL[] {jar}, ClassLoader.getPlatformClassLoader())) {
      // libkc throws these by name, from KcNative's class loader, which can't read the
      // jar once it is closed: load them now.
      load("KcException", shared);
      load("KcPanicException", shared);
      Class<?> c;
      try {
        c = load("KcNative", shared);
      } catch (UnsatisfiedLinkError e) {
        if (e.getMessage() == null || !e.getMessage().contains("already loaded in another classloader")) throw e;
        throw (UnsatisfiedLinkError) new UnsatisfiedLinkError("libkc (" + library() + ") is already loaded in this "
            + "JVM, but not by kc-aa's shared class loader: something loaded io.github.barteo.kc.KcNative "
            + "itself, or an older kc-aa did; restart the JVM (gradle --stop for a Gradle daemon)").initCause(e);
      }
      Object entry = load("KcLibrary", shared).getConstructor(String.class, String.class)
          .newInstance(library(), jar.toString());
      // Registered only once libkc has loaded, so a failed load leaves nothing behind.
      MBeanServerFactory.createMBeanServer(DOMAIN).registerMBean(entry, NAME);
      loaded = library();
      return c;
    } catch (java.io.IOException | ReflectiveOperationException | javax.management.JMException e) {
      throw (LinkageError) new LinkageError("libkc: can't make its shared class loader from " + jar).initCause(e);
    }
  }

  private static void warnIfOther(String loaded) {
    String wanted = System.getProperty("kc.lib");
    if (wanted != null && !wanted.equals(loaded)) {
      System.err.println("kc: libkc is already loaded in this JVM from " + loaded + "; using it, not " + wanted
          + " (kc.lib)");
    }
  }

  private static Class<?> load(String simpleName, ClassLoader l) {
    try {
      return Class.forName(PKG + simpleName, true, l);
    } catch (ClassNotFoundException e) {
      throw (NoClassDefFoundError) new NoClassDefFoundError(PKG + simpleName + " in " + l).initCause(e);
    }
  }

  /**
   * {@code e} as this class loader's KcPanicException or KcException if it is another
   * class loader's, by class name; else {@code e}.
   */
  static RuntimeException translate(RuntimeException e) {
    for (Class<?> c = e.getClass(); c != null; c = c.getSuperclass()) {
      KcException k = null;
      if (c.getName().equals(PKG + "KcPanicException")) {
        k = new KcPanicException(e.getMessage());
      } else if (c.getName().equals(PKG + "KcException")) {
        k = new KcException(e.getMessage());
      }
      if (k != null) {
        k.setStackTrace(e.getStackTrace());
        return k;
      }
    }
    return e;
  }

  private static ObjectName objectName(String name) {
    try {
      return new ObjectName(name);
    } catch (javax.management.MalformedObjectNameException e) {
      throw new ExceptionInInitializerError(e);
    }
  }
}

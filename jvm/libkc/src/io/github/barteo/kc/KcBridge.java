package io.github.barteo.kc;

import java.lang.ref.PhantomReference;
import java.lang.ref.Reference;
import java.lang.ref.ReferenceQueue;
import java.net.URL;
import java.net.URLClassLoader;
import java.security.AccessController;
import java.security.PrivilegedAction;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

/**
 * The JNI binding of libkc (cmd/libkc in kotlin-compiler-go), of which a JVM has one.
 *
 * <p>A JVM lets one class loader load a native library, and a Go library can't be unloaded. So
 * the first {@link KcNative} to need the library defines this class in a class loader of its
 * own, which sees the JDK's classes only, and every later one, in whichever class loader,
 * finds it there through its thread: the thread's name is this class's, and its context class
 * loader this class's. The thread keeps the class, and so the library, for the life of the JVM.
 * Everything here is in the JDK's types, so that this class holds on to no other class loader.
 */
public final class KcBridge implements Runnable {
  /** This class's name, and its thread's. */
  public static final String NAME = "io.github.barteo.kc.KcBridge";

  /** The values {@link #call} stores in {@code status[0]}. */
  public static final int OK = 0, ERROR = 1, PANIC = 2;

  private static volatile String library;
  private static final ReferenceQueue<Object> dropped = new ReferenceQueue<>();
  private static final Map<Reference<?>, Map<Long, Map.Entry<Integer, byte[]>>> owners = new ConcurrentHashMap<>();

  private KcBridge() {}

  /**
   * Runs operation {@code op} on {@code request} and returns its response with {@code
   * status[0]} set to {@link #OK}, or the UTF-8 message of its failure with {@link #ERROR} or,
   * if kc panicked, {@link #PANIC}.
   */
  public static native byte[] call(int op, byte[] request, int[] status);

  /**
   * Loads the library from {@code path}, or as {@code kc} from {@code java.library.path} if
   * it is null, and starts the thread. {@code id} is what {@link #library} returns.
   */
  public static synchronized void load(String path, String id) {
    if (library != null) {
      throw new IllegalStateException("libkc is loaded");
    }
    if (path != null) {
      System.load(path);
    } else {
      System.loadLibrary("kc");
    }
    start();
    library = id;
  }

  /**
   * Returns a class loader for {@code urls} that sees the JDK's classes only. Made in a
   * privileged action, it keeps the protection domain of this class and not those of the
   * classes that are calling.
   */
  @SuppressWarnings("removal")
  public static ClassLoader loader(URL[] urls) {
    return AccessController.doPrivileged((PrivilegedAction<ClassLoader>) () -> new URLClassLoader(urls, null));
  }

  // A thread keeps the protection domains too, unless it is made in a privileged action.
  @SuppressWarnings("removal")
  private static void start() {
    AccessController.doPrivileged((PrivilegedAction<Void>) () -> {
      ThreadGroup group = Thread.currentThread().getThreadGroup();
      while (group.getParent() != null) {
        group = group.getParent();
      }
      Thread t = new Thread(group, new KcBridge(), NAME, 0, false);
      t.setDaemon(true);
      t.setContextClassLoader(KcBridge.class.getClassLoader());
      t.start();
      return null;
    });
  }

  /** What {@link #load} was told identifies the library. */
  public static String library() {
    return library;
  }

  /**
   * Returns a map of calls, an operation and its request each, that the thread makes once
   * {@code owner} is unreachable: what releases the handles {@code owner} holds. The caller
   * adds a call when it gets a handle and removes it when it releases the handle itself.
   */
  public static Map<Long, Map.Entry<Integer, byte[]>> track(Object owner) {
    Map<Long, Map.Entry<Integer, byte[]>> calls = new ConcurrentHashMap<>();
    owners.put(new PhantomReference<>(owner, dropped), calls);
    return calls;
  }

  @Override
  public void run() {
    for (;;) {
      try {
        Map<Long, Map.Entry<Integer, byte[]>> calls = owners.remove(dropped.remove());
        for (Map.Entry<Integer, byte[]> c : calls.values()) {
          call(c.getKey(), c.getValue(), new int[1]);
        }
      } catch (Throwable e) {
        // The thread is what keeps the library loaded: it doesn't end.
      }
    }
  }
}

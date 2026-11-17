import java.io.File;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.net.URL;
import java.net.URLClassLoader;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.util.Arrays;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Test of libkc in more than one class loader of a JVM, as a build tool's daemon has them: a
 * class loader for each class path, each with its own KcNative, at the same time and one
 * after another. Exits non-zero on any failure.
 *
 * <pre>java -Dkc.lib=/path/libkc.so -cp classes Loaders classes</pre>
 */
public class Loaders {
  // Operations of internal/kaops, and OpKaOpen's request for a module without a class path.
  static final int OPEN = 10, CLOSE = 12, HANDLES = 15;
  static final byte[] MODULE = {0, 1, 1, 'm', 1, 0, 0};

  static URL[] classes;

  /** A class loader with its own KcNative. */
  static final class Kc implements AutoCloseable {
    final URLClassLoader loader = new URLClassLoader(classes, ClassLoader.getPlatformClassLoader());
    final Class<?> kc;
    final Method call, atUnload, cancelAtUnload;

    Kc() throws Exception {
      kc = Class.forName("io.github.barteo.kc.KcNative", true, loader);
      call = kc.getMethod("call", int.class, byte[].class);
      atUnload = kc.getMethod("atUnload", long.class, int.class, byte[].class);
      cancelAtUnload = kc.getMethod("cancelAtUnload", long.class);
    }

    byte[] call(int op, byte[] request) throws Throwable {
      try {
        return (byte[]) call.invoke(null, op, request);
      } catch (InvocationTargetException e) {
        throw e.getCause();
      }
    }

    /** Opens a session, which libkc closes if the class loader is dropped, and returns the request that closes it. */
    byte[] open() throws Throwable {
      byte[] handle = call(OPEN, MODULE);
      atUnload.invoke(null, 1L, CLOSE, handle);
      return handle;
    }

    @Override
    public void close() throws Exception {
      loader.close();
    }
  }

  static void check(boolean ok, String what) {
    if (!ok) {
      System.out.println("FAIL: " + what);
      System.exit(1);
    }
  }

  public static void main(String[] args) throws Throwable {
    classes = new URL[] {new File(args[0]).toURI().toURL()};

    // 1. Two class loaders at once, each with its own classes, share the library.
    Kc a = new Kc(), b = new Kc();
    check(a.kc != b.kc, "the class loaders share KcNative");
    AtomicLong failures = new AtomicLong();
    Thread[] ts = new Thread[8];
    for (int t = 0; t < ts.length; t++) {
      Kc kc = t % 2 == 0 ? a : b;
      byte[] req = new byte[1 + t];
      Arrays.fill(req, (byte) t);
      ts[t] = new Thread(() -> {
        try {
          for (int i = 0; i < 20_000; i++) {
            if (!Arrays.equals(req, kc.call(2, req))) failures.incrementAndGet();
          }
        } catch (Throwable e) {
          e.printStackTrace();
          failures.incrementAndGet();
        }
      });
      ts[t].start();
    }
    for (Thread t : ts) t.join();
    ts = null;
    check(failures.get() == 0, failures.get() + " failed echoes");
    for (Kc kc : new Kc[] {a, b}) {
      try {
        kc.call(3, "boom".getBytes());
        check(false, "no exception for a panic");
      } catch (RuntimeException e) {
        check(e.getClass().getName().endsWith(".KcPanicException") && e.getClass().getClassLoader() == kc.loader,
            "a panic is a " + e.getClass() + " of " + e.getClass().getClassLoader());
      }
    }
    System.out.println("two class loaders at once: ok");

    // 2. A handle a class loader closes is closed once; one it leaves open is closed when
    // the class loader is dropped, the first one too, which loaded the library.
    check(b.call(HANDLES, new byte[0])[0] == 0, "handles are open at the start");
    a.open();
    byte[] hb = b.open();
    check(b.call(HANDLES, new byte[0])[0] == 2, "two sessions aren't two handles");
    b.cancelAtUnload.invoke(null, 1L);
    b.call(CLOSE, hb);
    check(b.call(HANDLES, new byte[0])[0] == 1, "closing a session leaves it open");
    a.close();
    a = null;
    int open = 1;
    for (int i = 0; i < 200 && open != 0; i++) {
      System.gc();
      Thread.sleep(50);
      open = b.call(HANDLES, new byte[0])[0];
    }
    check(open == 0, "a dropped class loader's handle stays open");
    System.out.println("a dropped class loader's handle: closed");

    // 3. A class loader made after every other is dropped.
    b.close();
    b = null;
    for (int i = 0; i < 5; i++) {
      System.gc();
      Thread.sleep(50);
    }
    Kc c = new Kc();
    check(Arrays.equals(new byte[] {7}, c.call(2, new byte[] {7})), "a later class loader's echo");
    System.out.println("a class loader after the others: ok");

    // 4. Another library file isn't loaded next to the first.
    String lib = System.getProperty("kc.lib");
    Path copy = Files.createTempFile("libkc", ".so");
    try {
      Files.copy(Path.of(lib), copy, StandardCopyOption.REPLACE_EXISTING);
      System.setProperty("kc.lib", copy.toString());
      try {
        new Kc();
        check(false, "a second library file loaded");
      } catch (UnsatisfiedLinkError e) {
        check(e.getMessage().contains(copy.toString()), "the error doesn't name the file: " + e.getMessage());
      }
    } finally {
      System.setProperty("kc.lib", lib);
      Files.delete(copy);
    }
    System.out.println("another library file: refused");
  }
}

// Loaders runs KSP on the kc backend in class loaders of its own making,
// in one JVM, as a build tool's daemon does: a class loader for each KSP
// class path, kept while a build runs and dropped when it ends.
//
//	Loaders KC_JAR RUN RUN RUN... (-Dkc.lib=... -Djava.awt.headless=true -XX:SoftRefLRUPolicyMSPerMB=0)
//
// Each RUN is CLASSPATH SRC_DIR OUT_DIR LIBS: RunKsp's arguments (RunKsp.kt)
// and the class path to run it on, which has RunKsp, the processors and
// the kc backend. The first two RUNs run at the same time, each in its own
// class loader, and both class loaders are kept until both end. Each later
// RUN runs in a new class loader after the ones before it are dropped. In the end every class loader is dropped, and
// libkc must close the sessions and models they left open; KC_JAR is the
// kc backend's jar, to ask libkc through. The JVM option has a garbage
// collection clear the JDK's soft references to a dropped class loader's
// classes, which otherwise keep it until memory runs short.
import java.io.File;
import java.lang.reflect.Method;
import java.net.URL;
import java.net.URLClassLoader;
import java.util.ArrayList;
import java.util.List;

public class Loaders {
  static URLClassLoader loader(String classpath) throws Exception {
    List<URL> urls = new ArrayList<>();
    for (String p : classpath.split(File.pathSeparator)) {
      if (!p.isEmpty()) urls.add(new File(p).toURI().toURL());
    }
    return new URLClassLoader(urls.toArray(new URL[0]), ClassLoader.getPlatformClassLoader());
  }

  /** Starts RUN args[i..i+3] in class loader l. */
  static Thread start(URLClassLoader l, String[] args, int i, List<String> failures) {
    Thread t = new Thread(() -> {
      try {
        Thread.currentThread().setContextClassLoader(l);
        Method run = l.loadClass("RunKspKt").getMethod("run", String[].class);
        Object code = run.invoke(null, (Object) new String[] {args[i + 1], args[i + 2], args[i + 3]});
        if (!code.equals(0)) throw new Exception("KSP exited with " + code);
      } catch (Throwable e) {
        e.printStackTrace();
        synchronized (failures) {
          failures.add(args[i + 2] + ": " + e);
        }
      }
    });
    t.start();
    return t;
  }

  /** The number of sessions and models open in libkc (OpKaHandles of internal/kaops). */
  static int handles(Method call) throws Exception {
    return ((byte[]) call.invoke(null, 15, new byte[0]))[0];
  }

  public static void main(String[] args) throws Exception {
    // AWT keeps the context class loader of the thread that first uses it, which the
    // IntelliJ core's classes do: this thread's, so that it is none of the dropped ones.
    java.awt.Toolkit.getDefaultToolkit().getSystemEventQueue();
    List<String> failures = new ArrayList<>();
    URLClassLoader la = loader(args[1]), lb = loader(args[5]);
    Thread a = start(la, args, 1, failures), b = start(lb, args, 5, failures);
    a.join();
    b.join();
    la.close();
    lb.close();
    a = b = null;
    la = lb = null;
    for (int i = 9; i < args.length; i += 4) {
      System.gc();
      try (URLClassLoader l = loader(args[i])) {
        start(l, args, i, failures).join();
      }
    }
    if (failures.isEmpty()) {
      try (URLClassLoader l = loader(args[0])) {
        Method call = l.loadClass("io.github.barteo.kc.KcNative").getMethod("call", int.class, byte[].class);
        int open = handles(call);
        for (int i = 0; i < 200 && open != 0; i++) {
          System.gc();
          Thread.sleep(100);
          open = handles(call);
        }
        if (open != 0) failures.add(open + " sessions and models of dropped class loaders stay open");
      }
    }
    failures.forEach(f -> System.err.println("Loaders: " + f));
    System.exit(failures.isEmpty() ? 0 : 1);
  }
}

import java.io.BufferedReader;
import java.io.File;
import java.io.FileDescriptor;
import java.io.FileOutputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.io.PrintStream;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.lang.reflect.Modifier;
import java.net.URL;
import java.net.URLClassLoader;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.FutureTask;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

/**
 * Runs the box tests bench/conformance compiled. Each line of stdin is a
 * test: an id, 1 if it uses kotlin-reflect, the class with box() and its
 * class path, separated by tabs. For each, it prints the id, a status and
 * a detail, separated by tabs: OK; FAIL and what box() returned; VERIFY,
 * LINKAGE or EXCEPTION and the throwable; NOBOX; or TIMEOUT, after which
 * it exits, since the test's thread can't be stopped.
 *
 *   java BoxRunner kotlin-stdlib.jar:kotlin-test.jar kotlin-reflect.jar timeoutSeconds
 */
public class BoxRunner {
    public static void main(String[] args) throws Exception {
        URL[] libs = urls(args[0]);
        URL[] withReflectLibs = urls(args[0] + File.pathSeparator + args[1]);
        long timeout = Long.parseLong(args[2]);
        ClassLoader platform = ClassLoader.getPlatformClassLoader();
        ClassLoader withStdlib = new URLClassLoader(libs, platform);
        ClassLoader withReflect = new URLClassLoader(withReflectLibs, platform);

        PrintStream out = new PrintStream(new FileOutputStream(FileDescriptor.out), true, StandardCharsets.UTF_8);
        PrintStream sink = new PrintStream(OutputStream.nullOutputStream());
        System.setOut(sink);
        System.setErr(sink);
        BufferedReader in = new BufferedReader(new InputStreamReader(System.in, StandardCharsets.UTF_8));
        for (String line; (line = in.readLine()) != null; ) {
            String[] f = line.split("\t", -1);
            String result = run(f[1].equals("1") ? withReflect : withStdlib, f[2], f[3], timeout);
            out.println(f[0] + "\t" + result);
            if (result.startsWith("TIMEOUT")) {
                Runtime.getRuntime().halt(0);
            }
        }
        Runtime.getRuntime().halt(0);
    }

    static String run(ClassLoader parent, String className, String classPath, long timeout) {
        Method box;
        URLClassLoader loader;
        try {
            loader = new URLClassLoader(urls(classPath), parent);
            Class<?> c = Class.forName(className, false, loader);
            box = c.getDeclaredMethod("box");
            if (!Modifier.isStatic(box.getModifiers())) {
                return "NOBOX\tbox() isn't static";
            }
            box.setAccessible(true);
        } catch (NoSuchMethodException e) {
            return "NOBOX\t" + oneLine(e.toString());
        } catch (ClassNotFoundException e) {
            return "NOBOX\t" + oneLine(e.toString());
        } catch (Throwable e) {
            return classify(e);
        }
        Method m = box;
        FutureTask<Object> task = new FutureTask<>(() -> m.invoke(null));
        Thread t = new Thread(task, "box");
        t.setDaemon(true);
        t.setContextClassLoader(loader);
        t.start();
        try {
            Object r = task.get(timeout, TimeUnit.SECONDS);
            if ("OK".equals(r)) {
                return "OK\t";
            }
            return "FAIL\t" + oneLine(String.valueOf(r));
        } catch (TimeoutException e) {
            return "TIMEOUT\t";
        } catch (ExecutionException e) {
            Throwable cause = e.getCause();
            if (cause instanceof InvocationTargetException && cause.getCause() != null) {
                cause = cause.getCause();
            }
            return classify(cause);
        } catch (InterruptedException e) {
            return "EXCEPTION\t" + oneLine(e.toString());
        }
    }

    static URL[] urls(String path) throws Exception {
        String[] entries = path.split(File.pathSeparator);
        URL[] urls = new URL[entries.length];
        for (int i = 0; i < entries.length; i++) {
            urls[i] = Path.of(entries[i]).toUri().toURL();
        }
        return urls;
    }

    static String classify(Throwable e) {
        if (e instanceof ExceptionInInitializerError && e.getCause() != null) {
            return "EXCEPTION\t" + describe(e.getCause()) + " (in an initializer)";
        }
        if (e instanceof VerifyError || e instanceof ClassFormatError) {
            return "VERIFY\t" + describe(e);
        }
        if (e instanceof LinkageError) {
            return "LINKAGE\t" + describe(e);
        }
        return "EXCEPTION\t" + describe(e);
    }

    /** The throwable's class, the first line of its message and where it was thrown. */
    static String describe(Throwable e) {
        String msg = e.getMessage() == null ? "" : e.getMessage();
        int nl = msg.indexOf('\n');
        if (nl >= 0) {
            msg = msg.substring(0, nl);
        }
        if (msg.length() > 200) {
            msg = msg.substring(0, 200) + "...";
        }
        String where = "";
        StackTraceElement[] st = e.getStackTrace();
        if (st.length > 0) {
            where = st[0].getClassName() + "." + st[0].getMethodName();
        }
        return e.getClass().getName() + "\t" + oneLine(msg) + "\t" + where;
    }

    static String oneLine(String s) {
        return s.replace("\r", " ").replace("\n", " ").replace("\t", " ");
    }
}

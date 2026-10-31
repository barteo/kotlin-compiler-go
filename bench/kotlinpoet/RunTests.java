import java.io.PrintStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import org.junit.runner.Description;
import org.junit.runner.JUnitCore;
import org.junit.runner.Request;
import org.junit.runner.notification.Failure;
import org.junit.runner.notification.RunListener;

/**
 * Runs the JUnit 4 test classes under a directory of compiled tests, one class at a
 * time, and writes one line per test to a file: class, method, PASS, FAIL or SKIP,
 * and for a failure the exception's class and the first line of its message, separated
 * by tabs. bench/kotlinpoet.sh runs it on kotlinc's and on kc's library classes.
 *
 * <p>Usage: java RunTests tests-dir out.tsv
 */
public class RunTests {
  public static void main(String[] args) throws Exception {
    Path root = Paths.get(args[0]);
    List<String> classes;
    try (Stream<Path> s = Files.walk(root)) {
      classes =
          s.map(p -> root.relativize(p).toString())
              .filter(n -> n.endsWith("Test.class") && !n.contains("$"))
              .map(n -> n.substring(0, n.length() - ".class".length()).replace('/', '.'))
              .sorted()
              .collect(Collectors.toList());
    }
    try (PrintStream out = new PrintStream(args[1], "UTF-8")) {
      for (String name : classes) {
        Class<?> cls;
        try {
          cls = Class.forName(name, false, RunTests.class.getClassLoader());
        } catch (Throwable t) {
          out.println(name + "\t-\tFAIL\t" + describe(t));
          continue;
        }
        if (java.lang.reflect.Modifier.isAbstract(cls.getModifiers()) || !hasTests(cls)) {
          continue; // a base class of tests, as Gradle skips them
        }
        JUnitCore core = new JUnitCore();
        List<String> lines = new ArrayList<>();
        core.addListener(
            new RunListener() {
              private boolean failed;

              @Override
              public void testStarted(Description d) {
                failed = false;
              }

              @Override
              public void testFailure(Failure f) {
                failed = true;
                lines.add(line(f.getDescription(), "FAIL\t" + describe(f.getException())));
              }

              @Override
              public void testAssumptionFailure(Failure f) {
                failed = true;
                lines.add(line(f.getDescription(), "SKIP\t" + describe(f.getException())));
              }

              @Override
              public void testIgnored(Description d) {
                lines.add(line(d, "SKIP\t\t"));
              }

              @Override
              public void testFinished(Description d) {
                if (!failed) {
                  lines.add(line(d, "PASS\t\t"));
                }
              }
            });
        core.run(Request.aClass(cls));
        lines.forEach(out::println);
        out.flush();
        System.err.println(name + ": " + lines.size() + " tests");
      }
    }
  }

  /**
   * Whether a class has tests for JUnit 4 to run: methods annotated @Test, its own or
   * inherited, or a runner of its own. Gradle skips the others, such as an open base
   * class.
   */
  static boolean hasTests(Class<?> cls) {
    if (cls.isAnnotationPresent(org.junit.runner.RunWith.class)
        || junit.framework.Test.class.isAssignableFrom(cls)) {
      return true;
    }
    for (Class<?> c = cls; c != null; c = c.getSuperclass()) {
      for (java.lang.reflect.Method m : c.getDeclaredMethods()) {
        if (m.isAnnotationPresent(org.junit.Test.class)) {
          return true;
        }
      }
    }
    return false;
  }

  static String line(Description d, String result) {
    String method = d.getMethodName() == null ? "-" : d.getMethodName();
    return d.getClassName() + "\t" + method + "\t" + result;
  }

  /** The exception's class and the first line of its message, without tabs. */
  static String describe(Throwable t) {
    String msg = String.valueOf(t.getMessage());
    int nl = msg.indexOf('\n');
    if (nl >= 0) {
      msg = msg.substring(0, nl);
    }
    if (msg.length() > 200) {
      msg = msg.substring(0, 200) + "...";
    }
    return t.getClass().getName() + "\t" + msg.replace('\t', ' ');
  }
}

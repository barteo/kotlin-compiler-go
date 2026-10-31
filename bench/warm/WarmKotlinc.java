import java.io.PrintStream;
import java.lang.reflect.Method;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import jdk.jfr.Configuration;
import jdk.jfr.Recording;

/**
 * Runs the Kotlin compiler several times in one JVM, the way a long-lived
 * daemon does, and reports each run's wall time. Later runs show warm-JIT
 * performance without JVM startup.
 *
 *   java -cp "warm:$KOTLIN_HOME/lib/*" WarmKotlinc <iterations> <kotlinc args...>
 * Separate modules with --next-module to time a sequential build in one JVM.
 *
 * "{run}" in an argument is replaced by the run number, so each run can
 * write its own file, e.g. -Xdump-perf=perf/{run}.json.
 *
 * With -Dcooldown=<seconds>, sleep before each iteration, outside its
 * measured time, while keeping the JVM and its JIT warm (default 0).
 *
 * With -Djfr=<file>, the second half of the runs (the ones the median is
 * taken from) is recorded with Java Flight Recorder's profiling settings,
 * sampling every millisecond.
 */
public class WarmKotlinc {
    public static void main(String[] argv) throws Exception {
        int iterations = Integer.parseInt(argv[0]);
        String[] args = Arrays.copyOfRange(argv, 1, argv.length);
        List<String[]> modules = new ArrayList<>();
        int first = 0;
        for (int i = 0; i <= args.length; i++) {
            if (i == args.length || args[i].equals("--next-module")) {
                if (first == i) throw new IllegalArgumentException("empty module arguments");
                modules.add(Arrays.copyOfRange(args, first, i));
                first = i + 1;
            }
        }
        Class<?> cls = Class.forName("org.jetbrains.kotlin.cli.jvm.K2JVMCompiler");
        Method exec = cls.getMethod("exec", PrintStream.class, String[].class);
        long cooldownMillis = Math.round(Double.parseDouble(System.getProperty("cooldown", "0")) * 1000);
        String jfr = System.getProperty("jfr");
        Recording recording = null;

        double[] ms = new double[iterations];
        for (int i = 0; i < iterations; i++) {
            Thread.sleep(cooldownMillis);
            if (jfr != null && i == iterations / 2) {
                recording = new Recording(Configuration.getConfiguration("profile"));
                recording.enable("jdk.ExecutionSample").withPeriod(Duration.ofMillis(1));
                recording.start();
            }
            long start = System.nanoTime();
            for (String[] module : modules) {
                Object compiler = cls.getConstructor().newInstance();
                String[] runArgs = new String[module.length];
                for (int j = 0; j < module.length; j++) {
                    runArgs[j] = module[j].replace("{run}", String.valueOf(i + 1));
                }
                Object exit = exec.invoke(compiler, System.err, (Object) runArgs);
                if (!"OK".equals(exit.toString())) {
                    System.err.println("compilation failed: " + exit);
                    System.exit(1);
                }
            }
            ms[i] = (System.nanoTime() - start) / 1e6;
            System.out.printf("run %d: %.0f ms%n", i + 1, ms[i]);
        }
        if (recording != null) {
            recording.stop();
            recording.dump(Path.of(jfr));
        }
        // Median of the second half, once the JIT has warmed up.
        double[] warm = Arrays.copyOfRange(ms, iterations / 2, iterations);
        Arrays.sort(warm);
        System.out.printf("warm median: %.0f ms%n", warm[warm.length / 2]);
    }
}

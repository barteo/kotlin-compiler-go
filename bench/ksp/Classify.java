import java.nio.file.*;
import java.util.*;
import jdk.jfr.consumer.*;

// Classify splits the samples of a KspBench recording (bench.jfr) by phase,
// found from the stack (a processor's process, its provider's create, or
// the rest of KotlinSymbolProcessing.execute), and within a phase by the
// leaf-most frame that isn't JDK or Kotlin standard library code. Native
// samples under KcNative.call are time in libkc (Go); time on Go's own
// threads (the checker's workers) isn't sampled.
// It also prints how long the recording ran, and how much of that GC
// pauses and safepoints took.
//
//	java bench/ksp/Classify.java bench.jfr
public class Classify {
    public static void main(String[] args) throws Exception {
        Map<String, Map<String, Integer>> byPhase = new TreeMap<>();
        Map<String, Integer> setupSteps = new HashMap<>();
        Map<String, Integer> topAnalysis = new HashMap<>();
        Map<String, Integer> topProc = new HashMap<>();
        Map<String, Double> waits = new HashMap<>();
        int total = 0, gcs = 0;
        double gcPause = 0, safepoint = 0;
        java.time.Instant first = null, last = null;
        for (RecordedEvent e : RecordingFile.readAllEvents(Paths.get(args[0]))) {
            String type = e.getEventType().getName();
            if (first == null || e.getStartTime().isBefore(first)) first = e.getStartTime();
            if (last == null || e.getEndTime().isAfter(last)) last = e.getEndTime();
            if (type.equals("jdk.GarbageCollection")) {
                gcs++;
                gcPause += e.getDuration("sumOfPauses").toNanos() / 1e6;
                continue;
            }
            if (type.equals("jdk.SafepointBegin")) {
                safepoint += e.getDuration().toNanos() / 1e6;
                continue;
            }
            if (type.equals("jdk.ThreadPark") || type.equals("jdk.JavaMonitorEnter") || type.equals("jdk.JavaMonitorWait")
                || type.equals("jdk.ThreadSleep") || type.equals("jdk.FileRead") || type.equals("jdk.FileWrite")) {
                var t = e.getThread("eventThread");
                if (t == null || !"main".equals(t.getJavaName())) continue;
                double ms = e.getDuration().toNanos() / 1e6;
                String where = type.substring(4);
                if (e.getStackTrace() != null) {
                    int n = 0;
                    for (RecordedFrame f : e.getStackTrace().getFrames()) {
                        String c = f.getMethod().getType().getName();
                        if (c.startsWith("java.") || c.startsWith("jdk.") || c.startsWith("sun.") || c.startsWith("kotlin.")) continue;
                        where += " < " + c.substring(c.lastIndexOf('.') + 1) + "." + f.getMethod().getName();
                        if (++n == 3) break;
                    }
                }
                waits.merge(where, ms, Double::sum);
                continue;
            }
            if (!type.equals("jdk.ExecutionSample") && !type.equals("jdk.NativeMethodSample")) continue;
            RecordedStackTrace st = e.getStackTrace();
            if (st == null) continue;
            String th = e.getThread("sampledThread").getJavaName();
            if (!"main".equals(th)) continue;
            total++;
            List<RecordedFrame> fr = st.getFrames(); // leaf first
            String phase = "other";
            int execIdx = -1;
            for (int i = 0; i < fr.size(); i++) {
                String c = fr.get(i).getMethod().getType().getName();
                String m = fr.get(i).getMethod().getName();
                if (c.equals("TimedProcessor") && m.equals("process")) { phase = "process"; break; }
                if (c.equals("TimedProvider") && m.equals("create")) { phase = "create"; break; }
                if (c.equals("com.google.devtools.ksp.impl.KotlinSymbolProcessing") && m.startsWith("execute")) { phase = "setup/between"; execIdx = i; break; }
            }
            if (st.isTruncated()) phase += " (truncated)";
            String cat = "jdk/stdlib";
            String leaf = "";
            for (RecordedFrame f : fr) {
                String c = f.getMethod().getType().getName();
                String k = category(c);
                if (k != null) { cat = k; leaf = c + "." + f.getMethod().getName(); break; }
            }
            byPhase.computeIfAbsent(phase, x -> new TreeMap<>()).merge(cat, 1, Integer::sum);
            if (phase.equals("setup/between") && execIdx > 0) {
                // The chain of frames just under execute names the step.
                StringBuilder sb = new StringBuilder();
                for (int i = execIdx - 1; i >= Math.max(0, execIdx - 3); i--) {
                    RecordedFrame f = fr.get(i);
                    sb.append(f.getMethod().getType().getName().replaceAll(".*\\.", "")).append('.').append(f.getMethod().getName()).append(" > ");
                }
                setupSteps.merge(sb.toString(), 1, Integer::sum);
            }
            if (phase.equals("process")) {
                if (cat.charAt(0) <= '2') topAnalysis.merge(leaf, 1, Integer::sum);
                else topProc.merge(leaf, 1, Integer::sum);
            }
        }
        System.out.println("main-thread samples " + total);
        if (first != null)
            System.out.printf("recorded %.0f ms; %d GCs paused %.0f ms; safepoints %.0f ms%n",
                java.time.Duration.between(first, last).toNanos() / 1e6, gcs, gcPause, safepoint);
        for (var en : byPhase.entrySet()) {
            int n = en.getValue().values().stream().mapToInt(i -> i).sum();
            System.out.printf("%-26s %6d (%.0f%%)%n", en.getKey(), n, 100.0 * n / total);
            for (var c : en.getValue().entrySet()) System.out.printf("    %-40s %6d (%.0f%% of phase)%n", c.getKey(), c.getValue(), 100.0 * c.getValue() / n);
        }
        System.out.println("main thread waiting (ms):");
        waits.entrySet().stream().sorted((a, b) -> Double.compare(b.getValue(), a.getValue())).limit(12)
            .forEach(w -> System.out.printf("    %6.0f %s%n", w.getValue(), w.getKey()));
        System.out.println("setup steps:"); top(setupSteps, 12).forEach(x -> System.out.println("    " + x));
        System.out.println("process, analysis leaves:"); top(topAnalysis, 12).forEach(x -> System.out.println("    " + x));
        System.out.println("process, processor leaves:"); top(topProc, 12).forEach(x -> System.out.println("    " + x));
    }

    static String category(String c) {
        if (c.equals("io.github.barteo.kc.KcNative")) return "0 libkc (Go)";
        if (c.startsWith("io.github.barteo.kc.")) return "1 kc backend (JVM)";
        if (c.startsWith("ksp.") || c.startsWith("org.jetbrains.kotlin.") || c.startsWith("com.intellij.")) return "1 Analysis API / IntelliJ core";
        if (c.startsWith("com.google.devtools.ksp.")) return "2 KSP symbol layer";
        if (c.startsWith("dagger.spi.internal.shaded.androidx.")) return "3 XProcessing (KSP adapter)";
        if (c.startsWith("androidx.room.compiler.processing.")) return "3 XProcessing (KSP adapter)";
        if (c.startsWith("dagger.")) return "4 Dagger";
        if (c.startsWith("androidx.room.") || c.startsWith("org.antlr.")) return "4 Room";
        if (c.startsWith("org.sqlite.")) return "4 Room (sqlite-jdbc query verification)";
        if (c.startsWith("com.squareup.") || c.startsWith("com.google.googlejavaformat") || c.startsWith("com.google.common") || c.startsWith("com.google.errorprone")) return "5 JavaPoet/google-java-format/Guava";
        if (c.startsWith("Timed") || c.startsWith("KspBench")) return "6 harness";
        return null; // java.*, jdk.*, sun.*, kotlin.*: attribute to the caller
    }

    static List<String> top(Map<String, Integer> m, int n) {
        return m.entrySet().stream().sorted((a, b) -> b.getValue() - a.getValue()).limit(n).map(e -> e.getValue() + " " + e.getKey()).toList();
    }
}

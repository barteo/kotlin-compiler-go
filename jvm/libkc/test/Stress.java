import io.github.barteo.kc.KcException;
import io.github.barteo.kc.KcNative;
import io.github.barteo.kc.KcPanicException;
import java.util.Arrays;
import java.util.SplittableRandom;
import java.util.concurrent.atomic.AtomicLong;

/**
 * Stress test of the libkc JNI bridge: concurrent calls with GC pressure, Go panics as
 * exceptions, and the JVM's own SIGSEGV uses (implicit null checks, stack banging) after the
 * Go runtime has installed its signal handlers. Exits non-zero on any failure.
 *
 * <pre>java -Dkc.lib=/path/libkc.so -cp classes Stress [threads] [callsPerThread]</pre>
 */
public class Stress {
  static final class Box {
    int v;
  }

  static volatile Box nullBox;
  static int sink;

  public static void main(String[] args) throws Exception {
    int threads = args.length > 0 ? Integer.parseInt(args[0]) : 32;
    int calls = args.length > 1 ? Integer.parseInt(args[1]) : 31250;
    AtomicLong failures = new AtomicLong(), panics = new AtomicLong();

    // 1. Concurrent echoes of random payloads, with a panic every 1000th call.
    Thread garbage = new Thread(() -> {
      byte[][] keep = new byte[64][];
      for (int i = 0; !Thread.currentThread().isInterrupted(); i++) {
        keep[i & 63] = new byte[1 << (i % 17)];
      }
    });
    garbage.setDaemon(true);
    garbage.start();
    long t0 = System.nanoTime();
    Thread[] ts = new Thread[threads];
    for (int t = 0; t < threads; t++) {
      final int seed = t;
      ts[t] = new Thread(() -> {
        SplittableRandom r = new SplittableRandom(seed);
        for (int i = 0; i < calls; i++) {
          if (i % 1000 == 999) {
            try {
              KcNative.call(KcNative.OP_PANIC, ("boom " + seed).getBytes());
              failures.incrementAndGet();
            } catch (KcPanicException e) {
              if (!e.getMessage().contains("boom " + seed)) failures.incrementAndGet();
              panics.incrementAndGet();
            }
            continue;
          }
          byte[] req = new byte[r.nextInt(i % 5000 == 0 ? 1 << 20 : 4096)];
          r.nextBytes(req);
          if (!Arrays.equals(req, KcNative.call(KcNative.OP_ECHO, req))) failures.incrementAndGet();
        }
      });
      ts[t].start();
    }
    for (Thread t : ts) t.join();
    garbage.interrupt();
    long total = (long) threads * calls;
    System.out.printf("concurrent: %d calls on %d threads in %.0f ms, %d panics caught%n",
        total, threads, (System.nanoTime() - t0) / 1e6, panics.get());

    // 2. A request error is a KcException, not a panic.
    try {
      KcNative.call(9999, new byte[0]);
      failures.incrementAndGet();
    } catch (KcPanicException e) {
      failures.incrementAndGet();
    } catch (KcException e) {
      // expected
    }

    // 3. Implicit null checks: JIT-compiled code dereferences null, the JVM's SIGSEGV
    // handler turns it into a NullPointerException. With the Go runtime loaded the
    // signal must still reach the JVM.
    int npes = 0;
    for (int i = 0; i < 2_000_000; i++) {
      try {
        sink += nullBox.v;
      } catch (NullPointerException e) {
        npes++;
      }
    }
    if (npes != 2_000_000) failures.incrementAndGet();
    System.out.println("implicit null checks: " + npes + " NullPointerExceptions");

    // 4. Stack overflow, which the JVM detects with guard pages and SIGSEGV, in a thread
    // that also calls into Go.
    for (int i = 0; i < 20; i++) {
      KcNative.call(KcNative.OP_ECHO, new byte[8]);
      try {
        recurse(0);
        failures.incrementAndGet();
      } catch (StackOverflowError e) {
        // expected
      }
    }
    System.out.println("stack overflows: recovered 20");

    // 5. The cost of one call with a small payload, on one thread, after warm-up.
    byte[] small = new byte[16];
    for (int i = 0; i < 200_000; i++) KcNative.call(KcNative.OP_ECHO, small);
    int n = 1_000_000;
    long t1 = System.nanoTime();
    for (int i = 0; i < n; i++) KcNative.call(KcNative.OP_ECHO, small);
    System.out.printf("per call: %.0f ns (16-byte echo, one thread)%n", (System.nanoTime() - t1) / (double) n);

    if (failures.get() != 0) {
      System.out.println("FAIL: " + failures.get() + " failures");
      System.exit(1);
    }
    System.out.println("PASS");
  }

  static int recurse(int d) {
    return recurse(d + 1) + 1;
  }
}

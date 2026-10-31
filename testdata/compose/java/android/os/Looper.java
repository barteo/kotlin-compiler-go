package android.os;

// Android's Looper, for the Compose runtime's main thread checks: the main
// looper's thread is the one that first asks for it.
public final class Looper {
    private static Looper main;
    private final Thread thread;

    private Looper(Thread thread) {
        this.thread = thread;
    }

    public static synchronized Looper getMainLooper() {
        if (main == null) {
            main = new Looper(Thread.currentThread());
        }
        return main;
    }

    public Thread getThread() {
        return thread;
    }
}

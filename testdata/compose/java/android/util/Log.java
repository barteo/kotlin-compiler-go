package android.util;

// Android's Log, which the Compose runtime logs errors to.
public final class Log {
    public static int e(String tag, String msg, Throwable tr) {
        System.err.println(tag + ": " + msg);
        if (tr != null) {
            tr.printStackTrace();
        }
        return 0;
    }
}

public class J {
    public static <T> JavaBox<T> wrap(T a) { return new JavaBox<T>(a); }
    public static <T> T transform(T a) { return a; }
    public static <T> java.util.List<T> listOf(T a) { return null; }
}

public class JavaBox<T> {
    public JavaBox(T b) { a = b; }
    public T a;
    public static <T> JavaBox<T> same(JavaBox<T> a) { return a; }
}

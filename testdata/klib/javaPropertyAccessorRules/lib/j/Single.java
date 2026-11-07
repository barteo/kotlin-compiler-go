package j;
public class Single<T> {
    private final T v;
    public Single(T v) { this.v = v; }
    public T blockingGet() { return v; }
}

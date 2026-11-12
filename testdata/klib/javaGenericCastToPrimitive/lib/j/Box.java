package j;
public class Box<T> {
    private final T v;
    public Box(T v) { this.v = v; }
    public T getValue() { return v; }
}

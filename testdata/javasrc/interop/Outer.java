package interop;

// An inner class, whose instances belong to an Outer, and one with type
// parameters of its own; a protected constructor, for the package.
public class Outer<T> {
    private final T value;
    public Outer(T value) { this.value = value; }
    protected Outer() { this(null); }
    public class Inner<Z> {
        private final Z extra;
        public Inner(Z extra) { this.extra = extra; }
        public T getValue() { return value; }
        public Z getExtra() { return extra; }
    }
    public class Plain {
        public String describe() { return "plain of " + value; }
    }
}

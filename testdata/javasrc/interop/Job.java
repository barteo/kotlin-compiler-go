package interop;

public interface Job<T> extends Cloneable {
    T run();

    Job<T> clone();
}

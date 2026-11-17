package interop;

public interface Fn<T, R> {
    R apply(T t);
}

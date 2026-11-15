package interop;

import java.util.concurrent.Callable;

// A value from a callable, as a reactive library's single value source.
public class Source<T> {
    private final T value;

    private Source(T value) {
        this.value = value;
    }

    public static <T> Source<T> of(T value) {
        return new Source<T>(value);
    }

    public static <T> Source<T> fromCallable(Callable<? extends T> callable) {
        try {
            return new Source<T>(callable.call());
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    public T get() {
        return value;
    }
}

public interface Computable<T, E extends Throwable> {
    T compute() throws E;
}

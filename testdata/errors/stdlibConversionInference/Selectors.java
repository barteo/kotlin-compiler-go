public class Selectors {
    public static <T> T select(T a, T b) {
        return a;
    }

    public static <T, E extends Throwable> T compute(Computable<T, E> action) throws E {
        return action.compute();
    }
}

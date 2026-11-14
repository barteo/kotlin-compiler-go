package interop;

import java.util.function.BiFunction;

// Generic methods whose type parameter a null argument infers: Kotlin's
// Nothing!, which takes null and returns.
public class Nulls {
    public <T> T set(String key, T t) {
        return t;
    }

    public interface Visitor<D> {
    }

    public <D, R> R accept(Visitor<R> visitor, D data) {
        return null;
    }

    public <K, V> String remap(K key, BiFunction<? super K, ? super V, ? extends V> f) {
        return "remap " + f.apply(key, null);
    }
}

package p;

import java.util.HashMap;
import java.util.Map;

public class Conf {
    private final Map<Key<?>, Object> values = new HashMap<>();

    public <T> void put(Key<T> key, T value) { values.put(key, value); }

    @SuppressWarnings("unchecked")
    public <T> T get(Key<T> key) { return (T) values.get(key); }
}

package util;

import androidx.annotation.NonNull;

import java.util.AbstractMap;
import java.util.HashMap;
import java.util.Map;
import java.util.Set;

public class DefaultMap<K, V> extends AbstractMap<K, V> {
    private final Map<K, V> map = new HashMap<>();
    private final V fallback;

    public DefaultMap(V fallback) {
        this.fallback = fallback;
    }

    @NonNull
    @Override
    public Set<Map.Entry<K, V>> entrySet() {
        return map.entrySet();
    }

    @Override
    public V put(K key, V value) {
        return map.put(key, value);
    }

    @NonNull
    @Override
    public V get(Object key) {
        V v = map.get(key);
        return v == null ? fallback : v;
    }
}

package android.util;

import java.util.Objects;
import java.util.TreeMap;

// Android's SparseLongArray, for the parcelize tests: a map from int keys,
// in order of the keys.
public class SparseLongArray {
    private final TreeMap<Integer, Long> map = new TreeMap<>();

    public SparseLongArray() {}

    public SparseLongArray(int initialCapacity) {}

    public int size() { return map.size(); }

    public int keyAt(int index) { return (Integer) map.keySet().toArray()[index]; }

    public long valueAt(int index) { return (Long) map.values().toArray()[index]; }

    public void put(int key, long value) { map.put(key, value); }

    public long get(int key) { return map.get(key); }

    @Override
    public boolean equals(Object o) {
        return o instanceof SparseLongArray && ((SparseLongArray) o).map.equals(map);
    }

    @Override
    public int hashCode() {
        return Objects.hashCode(map);
    }

    @Override
    public String toString() {
        return map.toString();
    }
}

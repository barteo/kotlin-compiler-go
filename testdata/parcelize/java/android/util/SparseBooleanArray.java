package android.util;

import java.util.Objects;
import java.util.TreeMap;

// Android's SparseBooleanArray, for the parcelize tests: a map from int keys,
// in order of the keys.
public class SparseBooleanArray {
    private final TreeMap<Integer, Boolean> map = new TreeMap<>();

    public SparseBooleanArray() {}

    public SparseBooleanArray(int initialCapacity) {}

    public int size() { return map.size(); }

    public int keyAt(int index) { return (Integer) map.keySet().toArray()[index]; }

    public boolean valueAt(int index) { return (Boolean) map.values().toArray()[index]; }

    public void put(int key, boolean value) { map.put(key, value); }

    public boolean get(int key) { return map.get(key); }

    @Override
    public boolean equals(Object o) {
        return o instanceof SparseBooleanArray && ((SparseBooleanArray) o).map.equals(map);
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

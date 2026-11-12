package android.util;

import java.util.Objects;
import java.util.TreeMap;

// Android's SparseIntArray, for the parcelize tests: a map from int keys,
// in order of the keys.
public class SparseIntArray {
    private final TreeMap<Integer, Integer> map = new TreeMap<>();

    public SparseIntArray() {}

    public SparseIntArray(int initialCapacity) {}

    public int size() { return map.size(); }

    public int keyAt(int index) { return (Integer) map.keySet().toArray()[index]; }

    public int valueAt(int index) { return (Integer) map.values().toArray()[index]; }

    public void put(int key, int value) { map.put(key, value); }

    public int get(int key) { return map.get(key); }

    @Override
    public boolean equals(Object o) {
        return o instanceof SparseIntArray && ((SparseIntArray) o).map.equals(map);
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

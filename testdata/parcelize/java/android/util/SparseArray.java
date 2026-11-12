package android.util;

import java.util.Objects;
import java.util.TreeMap;

// Android's SparseArray, for the parcelize tests: a map from int keys, in
// order of the keys.
public class SparseArray<E> {
    private final TreeMap<Integer, E> map = new TreeMap<>();

    public SparseArray() {}

    public SparseArray(int initialCapacity) {}

    public int size() { return map.size(); }

    public int keyAt(int index) { return (Integer) map.keySet().toArray()[index]; }

    @SuppressWarnings("unchecked")
    public E valueAt(int index) { return (E) map.values().toArray()[index]; }

    public void put(int key, E value) { map.put(key, value); }

    public E get(int key) { return map.get(key); }

    @Override
    public boolean equals(Object o) {
        return o instanceof SparseArray && ((SparseArray<?>) o).map.equals(map);
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

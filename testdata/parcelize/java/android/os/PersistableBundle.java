package android.os;

import java.util.Objects;
import java.util.TreeMap;

// Android's PersistableBundle, for the parcelize tests: a map of strings. As
// Android's, it is Parcelable, which the plugin doesn't use: it writes a
// PersistableBundle with writePersistableBundle.
public final class PersistableBundle implements Parcelable {
    private final TreeMap<String, String> map = new TreeMap<>();

    public void putString(String key, String value) {
        map.put(key, value);
    }

    public String getString(String key) {
        return map.get(key);
    }

    PersistableBundle copy() {
        PersistableBundle b = new PersistableBundle();
        b.map.putAll(map);
        return b;
    }

    @Override
    public int describeContents() {
        return 0;
    }

    @Override
    public void writeToParcel(Parcel dest, int flags) {
        throw new UnsupportedOperationException("PersistableBundle.writeToParcel");
    }

    @Override
    public boolean equals(Object o) {
        return o instanceof PersistableBundle && ((PersistableBundle) o).map.equals(map);
    }

    @Override
    public int hashCode() {
        return Objects.hashCode(map);
    }

    @Override
    public String toString() {
        return "PersistableBundle" + map;
    }
}

package android.os;

import java.util.Objects;
import java.util.TreeMap;

// Android's Bundle, for the parcelize tests: a map of strings. As
// Android's, it is Parcelable, which the plugin doesn't use: it writes a
// Bundle with writeBundle.
public final class Bundle implements Parcelable {
    private final TreeMap<String, String> map = new TreeMap<>();

    public void putString(String key, String value) {
        map.put(key, value);
    }

    public String getString(String key) {
        return map.get(key);
    }

    Bundle copy() {
        Bundle b = new Bundle();
        b.map.putAll(map);
        return b;
    }

    @Override
    public int describeContents() {
        return 0;
    }

    @Override
    public void writeToParcel(Parcel dest, int flags) {
        throw new UnsupportedOperationException("Bundle.writeToParcel");
    }

    @Override
    public boolean equals(Object o) {
        return o instanceof Bundle && ((Bundle) o).map.equals(map);
    }

    @Override
    public int hashCode() {
        return Objects.hashCode(map);
    }

    @Override
    public String toString() {
        return "Bundle" + map;
    }
}

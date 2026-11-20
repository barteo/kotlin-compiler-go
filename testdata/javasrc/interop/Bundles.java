package interop;

import java.util.HashMap;
import java.util.Map;

// Bundles reads a value as Android's Bundle reads a Parcelable: with a
// method whose type parameter only its bound gives, and with one that
// takes the value's class.
public class Bundles {
    public interface Parcel {
        String name();
    }

    private final Map<String, Parcel> items = new HashMap<>();

    public void put(String key, Parcel p) { items.put(key, p); }

    @SuppressWarnings("unchecked")
    public <T extends Parcel> T get(String key) { return (T) items.get(key); }

    public <T> T get(String key, Class<T> clazz) { return clazz.cast(items.get(key)); }
}

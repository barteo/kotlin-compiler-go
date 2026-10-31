package android.os;

import java.io.Serializable;
import java.util.ArrayList;
import java.util.List;

// A fake of Android's Parcel, for the parcelize tests: it keeps the values
// written, in order, which reads return, and a log of the writes.
public final class Parcel {
    private final ArrayList<Object> values = new ArrayList<>();
    private final StringBuilder log = new StringBuilder();
    private int pos;

    // log returns the writes, as kind(value) separated by spaces.
    public String log() {
        return log.toString().trim();
    }

    private void put(String kind, Object v) {
        log.append(kind).append('(').append(v).append(") ");
        values.add(v);
    }

    private Object next() {
        return values.get(pos++);
    }

    public void writeInt(int v) { put("int", v); }
    public int readInt() { return (Integer) next(); }
    public void writeLong(long v) { put("long", v); }
    public long readLong() { return (Long) next(); }
    public void writeFloat(float v) { put("float", v); }
    public float readFloat() { return (Float) next(); }
    public void writeDouble(double v) { put("double", v); }
    public double readDouble() { return (Double) next(); }
    public void writeByte(byte v) { put("byte", v); }
    public byte readByte() { return (Byte) next(); }
    public void writeString(String v) { put("string", v); }
    public String readString() { return (String) next(); }

    public void writeStringList(List<String> v) {
        put("stringList", v == null ? null : new ArrayList<>(v));
    }

    @SuppressWarnings("unchecked")
    public ArrayList<String> createStringArrayList() {
        Object v = next();
        return v == null ? null : new ArrayList<>((List<String>) v);
    }

    public void writeStringArray(String[] v) {
        put("stringArray", v == null ? null : java.util.Arrays.asList(v.clone()));
    }

    @SuppressWarnings("unchecked")
    public String[] createStringArray() {
        Object v = next();
        return v == null ? null : ((List<String>) v).toArray(new String[0]);
    }

    public void writeSerializable(Serializable v) { put("serializable", v); }
    public Serializable readSerializable() { return (Serializable) next(); }

    // As Android's, the class's name and then what its writeToParcel
    // writes, which readParcelable reads with the class's CREATOR.
    public void writeParcelable(Parcelable p, int flags) {
        if (p == null) {
            put("parcelable", null);
            return;
        }
        put("parcelable", p.getClass().getName());
        p.writeToParcel(this, flags);
    }

    @SuppressWarnings("unchecked")
    public <T extends Parcelable> T readParcelable(ClassLoader loader) {
        String name = (String) next();
        if (name == null) {
            return null;
        }
        try {
            Object creator = Class.forName(name, true, loader).getField("CREATOR").get(null);
            return (T) ((Parcelable.Creator<?>) creator).createFromParcel(this);
        } catch (ReflectiveOperationException e) {
            throw new RuntimeException(e);
        }
    }

    // Android 13's, which takes the class too.
    public <T> T readParcelable(ClassLoader loader, Class<T> clazz) {
        return clazz.cast(readParcelable(loader));
    }
}

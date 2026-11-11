package android.os;

import android.util.Size;
import android.util.SizeF;
import android.util.SparseBooleanArray;
import java.io.FileDescriptor;
import java.io.Serializable;
import java.util.ArrayList;
import java.util.Arrays;
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

    // put logs v as shown and keeps value.
    private void put(String kind, Object shown, Object value) {
        log.append(kind).append('(').append(shown).append(") ");
        values.add(value);
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
        put("stringArray", v == null ? null : Arrays.asList(v.clone()));
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

    // The arrays of primitives, which, as Android's, write null too.
    public void writeIntArray(int[] v) { put("intArray", v == null ? null : Arrays.toString(v), v == null ? null : v.clone()); }
    public int[] createIntArray() { int[] v = (int[]) next(); return v == null ? null : v.clone(); }
    public void writeLongArray(long[] v) { put("longArray", v == null ? null : Arrays.toString(v), v == null ? null : v.clone()); }
    public long[] createLongArray() { long[] v = (long[]) next(); return v == null ? null : v.clone(); }
    public void writeFloatArray(float[] v) { put("floatArray", v == null ? null : Arrays.toString(v), v == null ? null : v.clone()); }
    public float[] createFloatArray() { float[] v = (float[]) next(); return v == null ? null : v.clone(); }
    public void writeDoubleArray(double[] v) { put("doubleArray", v == null ? null : Arrays.toString(v), v == null ? null : v.clone()); }
    public double[] createDoubleArray() { double[] v = (double[]) next(); return v == null ? null : v.clone(); }
    public void writeBooleanArray(boolean[] v) { put("booleanArray", v == null ? null : Arrays.toString(v), v == null ? null : v.clone()); }
    public boolean[] createBooleanArray() { boolean[] v = (boolean[]) next(); return v == null ? null : v.clone(); }
    public void writeByteArray(byte[] v) { put("byteArray", v == null ? null : Arrays.toString(v), v == null ? null : v.clone()); }
    public byte[] createByteArray() { byte[] v = (byte[]) next(); return v == null ? null : v.clone(); }
    public void writeCharArray(char[] v) { put("charArray", v == null ? null : Arrays.toString(v), v == null ? null : v.clone()); }
    public char[] createCharArray() { char[] v = (char[]) next(); return v == null ? null : v.clone(); }

    public void writeBundle(Bundle v) { put("bundle", v == null ? null : v.copy()); }
    public Bundle readBundle(ClassLoader loader) { return (Bundle) next(); }
    public void writePersistableBundle(PersistableBundle v) { put("persistableBundle", v == null ? null : v.copy()); }
    public PersistableBundle readPersistableBundle(ClassLoader loader) { return (PersistableBundle) next(); }

    public void writeSize(Size v) { put("size", v); }
    public Size readSize() { return (Size) next(); }
    public void writeSizeF(SizeF v) { put("sizeF", v); }
    public SizeF readSizeF() { return (SizeF) next(); }

    public void writeSparseBooleanArray(SparseBooleanArray v) {
        SparseBooleanArray copy = null;
        if (v != null) {
            copy = new SparseBooleanArray();
            for (int i = 0; i < v.size(); i++) {
                copy.put(v.keyAt(i), v.valueAt(i));
            }
        }
        put("sparseBooleanArray", copy);
    }

    public SparseBooleanArray readSparseBooleanArray() { return (SparseBooleanArray) next(); }

    // Binders pass by reference.
    public void writeStrongBinder(IBinder v) { put("binder", v); }
    public IBinder readStrongBinder() { return (IBinder) next(); }
    public void writeBinderArray(IBinder[] v) { put("binderArray", v == null ? null : Arrays.toString(v), v == null ? null : v.clone()); }
    public IBinder[] createBinderArray() { IBinder[] v = (IBinder[]) next(); return v == null ? null : v.clone(); }
    public void writeBinderList(List<IBinder> v) { put("binderList", v == null ? null : new ArrayList<>(v)); }

    @SuppressWarnings("unchecked")
    public ArrayList<IBinder> createBinderArrayList() {
        Object v = next();
        return v == null ? null : new ArrayList<>((List<IBinder>) v);
    }

    // A file descriptor passes by reference, but Android reads it back as
    // a ParcelFileDescriptor.
    public void writeFileDescriptor(FileDescriptor v) { put("fileDescriptor", v == null ? null : "fd", v); }
    public ParcelFileDescriptor readFileDescriptor() { next(); return new ParcelFileDescriptor(); }
}

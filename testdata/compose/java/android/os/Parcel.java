package android.os;

// Android's Parcel, for the Compose runtime's parcelable state classes,
// which the compose tests don't write.
public final class Parcel {
    private static UnsupportedOperationException unsupported() {
        return new UnsupportedOperationException("Parcel");
    }

    public void writeInt(int v) { throw unsupported(); }
    public int readInt() { throw unsupported(); }
    public void writeLong(long v) { throw unsupported(); }
    public long readLong() { throw unsupported(); }
    public void writeFloat(float v) { throw unsupported(); }
    public float readFloat() { throw unsupported(); }
    public void writeDouble(double v) { throw unsupported(); }
    public double readDouble() { throw unsupported(); }
    public void writeValue(Object v) { throw unsupported(); }
    public Object readValue(ClassLoader loader) { throw unsupported(); }
}

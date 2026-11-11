package android.util;

// Android's SizeF, for the parcelize tests.
public final class SizeF {
    private final float width, height;

    public SizeF(float width, float height) {
        this.width = width;
        this.height = height;
    }

    public float getWidth() { return width; }
    public float getHeight() { return height; }

    @Override
    public boolean equals(Object o) {
        return o instanceof SizeF && ((SizeF) o).width == width && ((SizeF) o).height == height;
    }

    @Override
    public int hashCode() {
        return Float.hashCode(width) * 31 + Float.hashCode(height);
    }

    @Override
    public String toString() {
        return width + "x" + height;
    }
}

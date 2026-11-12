package android.util;

// Android's Size, for the parcelize tests.
public final class Size {
    private final int width, height;

    public Size(int width, int height) {
        this.width = width;
        this.height = height;
    }

    public int getWidth() { return width; }
    public int getHeight() { return height; }

    @Override
    public boolean equals(Object o) {
        return o instanceof Size && ((Size) o).width == width && ((Size) o).height == height;
    }

    @Override
    public int hashCode() {
        return width * 31 + height;
    }

    @Override
    public String toString() {
        return width + "x" + height;
    }
}

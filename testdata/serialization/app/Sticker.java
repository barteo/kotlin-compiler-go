package app;

import kotlinx.serialization.Serializable;

// A Java class of the program that names its serializer, a class.
@Serializable(with = StickerSerializer.class)
public class Sticker {
    private final String text;

    public Sticker(String text) {
        this.text = text;
    }

    public String getText() {
        return text;
    }

    @Override
    public boolean equals(Object o) {
        return o instanceof Sticker && ((Sticker) o).text.equals(text);
    }

    @Override
    public int hashCode() {
        return text.hashCode();
    }

    @Override
    public String toString() {
        return "Sticker(" + text + ")";
    }
}

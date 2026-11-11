package app;

import java.io.Serializable;

// A Java class that names its serializer with the annotation's full name,
// as Serializable is java.io's here.
@kotlinx.serialization.Serializable(with = TagSerializer.class)
public class Tag implements Serializable {
    public final String text;

    public Tag(String text) {
        this.text = text;
    }

    @Override
    public String toString() {
        return "Tag(" + text + ")";
    }
}

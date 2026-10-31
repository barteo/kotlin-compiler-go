package util;

import app.*;
import java.util.*;

// A generic Java class bounded by a Kotlin class, in another package.
public final class Registry<S extends Shape> {
    public static final String PREFIX = Square.KIND + "#" + Square.SIDES;

    private final Map<String, S> shapes = new LinkedHashMap<>();

    public void add(S s) {
        shapes.put(PREFIX + s.getName(), s);
    }

    public Collection<S> all() {
        return shapes.values();
    }

    public S first() {
        return shapes.values().iterator().next();
    }
}

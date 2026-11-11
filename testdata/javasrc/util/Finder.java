package util;

import java.util.Collection;
import java.util.function.Predicate;

public class Finder {
    public static <T> T find(Collection<T> items, Predicate<T> predicate) {
        return find(items, predicate, null);
    }

    public static <T> T find(Collection<T> items, Predicate<T> predicate, T fallback) {
        for (T item : items) {
            if (predicate.test(item)) {
                return item;
            }
        }
        return fallback;
    }
}

package lib;

import org.jetbrains.annotations.NotNull;

public class G<T extends @NotNull Object> {
    public T f;
    public T foo(T a) { return a; }
    public static <U extends @NotNull Comparable<U>> U cmp(U a) { return a; }
    public static <U extends Object & @NotNull Comparable<U>> U two(U a) { return a; }
}

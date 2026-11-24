import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

public class M {
    public static <T extends @NotNull Object> T id(T a) { return a; }
    public static <T extends @NotNull Comparable<T>> T cmp(T a) { return a; }
    public static <T extends @Nullable Number> T num(T a) { return a; }
    public static <T extends Object & @NotNull Comparable<T>> T two(T a) { return a; }
    public static <T> T plain(T a) { return a; }
}

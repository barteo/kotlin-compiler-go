import org.jetbrains.annotations.NotNull;

public interface I<T extends @NotNull Object> {
    T apply(T a);
}

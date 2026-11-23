import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

public class G<T extends @NotNull Object> {
    public T f;
    public T foo(T a) { return a; }
    public JavaBox<T> box(JavaBox<T> a) { return a; }
    public void maybe(@Nullable T a) {}
}

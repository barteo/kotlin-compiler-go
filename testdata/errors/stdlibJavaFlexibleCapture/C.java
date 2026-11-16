import java.util.concurrent.Future;
import org.jetbrains.annotations.*;

public class C {
    public static <F> F await(Future<F> future) { return null; }
    public static <T> T exact(T a, T b) { return a; }
    public static <T> @NotNull T notNull(T t) { return t; }
}

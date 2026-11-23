import java.util.List;
import org.jetbrains.annotations.NotNull;

public class N {
    public static <T extends @NotNull Object> void nn(JavaBox<T> b) {}
    public static <T extends @NotNull Object> void nl(List<T> b) {}
    public static <T> void p(JavaBox<T> b) {}
    public static <T> void pl(List<T> b) {}
}

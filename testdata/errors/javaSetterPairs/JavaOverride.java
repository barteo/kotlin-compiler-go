import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

public class JavaOverride implements JavaBase {
    @Override @NotNull public String getFoo() { return ""; }
    @Override public void setFoo(@Nullable String foo) { }
}

import org.jetbrains.annotations.NotNull;
import org.jetbrains.annotations.Nullable;

public class JavaClass {
    public int getGeneric() { return 1; }
    public <T> void setGeneric(int value) { }

    public int getPlain() { return 1; }
    public void setPlain(int value) { }

    @NotNull public String getName() { return ""; }
    public void setName(@Nullable String name) { }
}

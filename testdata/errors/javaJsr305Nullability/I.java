import javax.annotation.Nonnull;
import javax.annotation.Nullable;

public interface I {
    @Nullable String get();
    @Nonnull String name();
    void put(@Nonnull String s);
    void opt(@Nullable String s);
}

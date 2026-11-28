import javax.annotation.CheckForNull;
import javax.annotation.Nonnull;
import javax.annotation.Nullable;
import javax.annotation.meta.When;

public class J {
    public J() {}
    public J(@Nonnull String s) {}

    public @Nullable String nullable() { return null; }
    public @CheckForNull String checkForNull() { return null; }
    public @Nonnull String nonnull() { return ""; }
    public @Nonnull(when = When.ALWAYS) String always() { return ""; }
    public @Nonnull(when = When.MAYBE) String maybe() { return null; }
    public @javax.annotation.Nonnull(when = javax.annotation.meta.When.NEVER) String never() { return null; }
    public @Nonnull(when = When.UNKNOWN) String unknown() { return null; }
    public @MaybeNull String nickname() { return null; }

    public void takeNullable(@Nullable String s) {}
    public void takeNonnull(@Nonnull String s) {}
    public void takeMaybe(@Nonnull(when = When.MAYBE) String s) {}

    public @Nullable String nullableField;
    public @Nonnull String nonnullField = "";
}

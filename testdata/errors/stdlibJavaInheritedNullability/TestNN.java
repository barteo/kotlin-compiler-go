import org.jetbrains.annotations.*;

public class TestNN {
    public interface JNotNull {
        @NotNull Object foo();
    }

    public static class JNullBase {
        @Nullable public Object foo() {
            return null;
        }
    }

    public static class JUnknownImpl extends JNullBase {
        public Object foo() {
            return this;
        }
    }

    public static class JUnknownVsNotNull extends JUnknownImpl implements JNotNull {
    }
}

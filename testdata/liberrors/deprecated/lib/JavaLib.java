package lib;

public class JavaLib {
    @kotlin.Deprecated(message = "gone", level = kotlin.DeprecationLevel.ERROR)
    public static int errMethod() { return 1; }

    @Deprecated
    @kotlin.Deprecated(message = "gone", level = kotlin.DeprecationLevel.HIDDEN)
    public static int hiddenMethod() { return 1; }

    @Deprecated
    public static int javaDeprecated() { return 1; }
}

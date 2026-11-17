public class JavaUtils {
    public static String foo1(int b) { return null; }
    public static int foo1(Integer b) { return 0; }
    public static String foo2(int b) { return null; }
    public static int foo2(@org.jetbrains.annotations.NotNull Integer b) { return 0; }
    public static String foo3(long b) { return null; }
    public static int foo3(int b) { return 0; }
    public static String foo4(long b) { return null; }
    public static int foo4(Integer b) { return 0; }
    public static String foo5(long b) { return null; }
    public static int foo5(@org.jetbrains.annotations.NotNull Integer b) { return 0; }
}

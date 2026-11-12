package j;
public class Asserts {
    public static String assertThat(boolean b) { return "prim " + b; }
    public static String assertThat(Boolean b) { return "boxed " + b; }
    public static String assertThat(Object o) { return "object " + o; }
}

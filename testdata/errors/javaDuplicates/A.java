// Broken Java, as kc reads it from source: members that javac rejects.
public class A {
    public String foo() { return ""; }
    public CharSequence foo() { return ""; }

    public static String bar() { return ""; }
    public static CharSequence bar() { return ""; }

    public String f;
    public String f = "";

    public static int baz(String x) { return 0; }
    public int baz(CharSequence x) { return 0; }
}

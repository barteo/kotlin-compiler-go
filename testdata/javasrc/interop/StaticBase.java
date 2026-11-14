package interop;

// A static method that an inner Kotlin subclass calls unqualified, before
// its outer class's member of the name.
public class StaticBase {
    public static String foo() {
        return "static foo";
    }
}

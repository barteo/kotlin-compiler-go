package interop;

// A Java interface whose members a Kotlin interface redeclares with a
// more specific result: the intersection override of an interface that
// extends both returns the more specific type (see use.kt).
public interface Labels {
    @Deprecated
    CharSequence label();

    @Deprecated
    String code();
}

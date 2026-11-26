package interop;

// A Java interface extending a Kotlin fun interface whose one abstract
// method is an Object method's redeclaration (see use.kt): it is a SAM
// interface too.
public interface JMatcher extends Matcher {
}

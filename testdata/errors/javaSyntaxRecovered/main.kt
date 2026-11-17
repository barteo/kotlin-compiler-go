// A Java source with syntax errors that Java PSI recovers from, a missing
// ; and a member it leaves out, is read without them, as kotlinc reads
// it: javac reports them.
fun eat(s: String) {}

fun test(s: String?) {
    eat(J.transform(s)) // ERROR type mismatch: inferred type is String? but String was expected
    J.count(java.util.ArrayList<String>())
    J().broken(1) // ERROR unresolved reference
}

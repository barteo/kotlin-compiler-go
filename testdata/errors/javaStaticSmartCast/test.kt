// A final static field of the program's Java sources is stable, as in
// K2: an is check or a cast smart casts it. A non-final one isn't.
fun isCast() {
    if (J.staticFinalJava is String) {
        J.staticFinalJava.length
    }
    if (J.staticNonFinalJava is String) {
        J.staticNonFinalJava.length // ERROR unresolved reference: length
    }
}

fun asCast() {
    J.staticFinalJava as String
    J.staticFinalJava.length
}

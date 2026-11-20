// Arguments that the star projection implies need no check: Map's V is
// out, Comparable's T in.
fun starImplied(x: Any) {
    x is Map<out Any?, Any?>
    x is Comparable<Nothing>
    x is Map<out Any?, String> // ERROR cannot check for instance of erased type: Map<out Any?, String>
    x is MutableList<Any?> // ERROR cannot check for instance of erased type: MutableList<Any?>
}

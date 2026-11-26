// A val that safe-casts a var no longer smart casts it once the var is
// assigned: val d = v as? S, then v = null.

interface S { val foo: Int }

fun g(a: Any?): Int {
    var v: Any? = a
    val d = v as? S
    v = null
    if (d != null) {
        return v.foo // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed
    }
    return 0
}

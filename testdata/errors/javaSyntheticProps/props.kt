// Java getters that make no synthetic property: one that overrides a
// Kotlin function, and one whose name can't start an identifier.

open class K {
    open fun getSomething1(): Int = 1
}

fun use(j: J) {
    j.getSomething1()
    j.something1 // ERROR unresolved
    j.other
    j.get1()
    j.`1` // ERROR unresolved
}

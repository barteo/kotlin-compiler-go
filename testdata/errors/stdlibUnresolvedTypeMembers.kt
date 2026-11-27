// As in K2, the members of a value whose written type doesn't resolve are
// unresolved, called, read or referred to; not those of an unresolved
// name or call, which is the error.

class Scope<T>(private val implClass: j.Class<T>, // ERROR unresolved or unsupported type: j.Class
    val d: Unknown) { // ERROR unresolved or unsupported type: Unknown
    fun foo(c: Collection<Any>) {
        c.filter(implClass::isInstance) // ERROR unresolved reference: isInstance
        implClass.cast(1) // ERROR unresolved reference: cast
        d.bar // ERROR unresolved reference: bar
        val e = d
        e.baz() // ERROR unresolved reference: baz
    }
}

fun top() {
    val z = unresolvedCall() // ERROR unresolved reference: unresolvedCall
    z.qux
    unresolvedName.quux // ERROR unresolved reference: unresolvedName
}

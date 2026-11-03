// A declaration can't expose in its signature a type less visible than
// itself. A member is as visible as its class at most.

internal open class Hidden
private class Secret
open class Base {
    protected class Guarded
    protected fun guarded(g: Guarded) = g
    fun leak(g: Guarded) {} // ERROR 'public' function exposes its 'protected' parameter type 'Base.Guarded'
}

fun ret() = Hidden() // ERROR 'public' function exposes its 'internal' return type 'Hidden'
fun param(h: Hidden) {} // ERROR 'public' function exposes its 'internal' parameter type 'Hidden'
fun Hidden.ext() {} // ERROR 'public' member exposes its 'internal' receiver type 'Hidden'
val prop: Secret? = null // ERROR 'public' property exposes its 'private-in-file' type 'Secret'
fun <T : Hidden> bound(t: T) {} // ERROR 'public' generic exposes its 'internal' parameter bound type 'Hidden'
class G<T>
fun arg(g: G<Hidden>) {} // ERROR 'public' function exposes its 'internal' parameter type argument 'Hidden'

class Sub : Hidden() // ERROR 'public' subclass exposes its 'internal' supertype 'Hidden'
typealias Alias = Hidden // ERROR 'public' typealias exposes its 'internal' expanded type 'Hidden'

class Ctor(val h: Hidden) // ERROR 'public' function exposes its 'internal' parameter type 'Hidden'
class Private private constructor(val h: Hidden) // ERROR 'public' property exposes its 'internal' type 'Hidden'

// As visible as the type, or less: no errors.
internal fun ok1(h: Hidden) = h
private fun ok2(s: Secret) = s
internal class InternalSub : Hidden()
class Outer {
    internal fun ok3(h: Hidden) = h
    private class Nested(val s: Secret)
}
internal class Box {
    fun ok4(h: Hidden) = h
}
class Derived : Base() {
    protected fun ok5(g: Guarded) = g
}

// A class may implement a less visible interface.
private interface Marker
class Marked : Marker

// Local classes and object expressions aren't checked; an object's type
// is its supertype.
fun local() {
    class Local
    fun make() = Local()
}
private val anon = object : Hidden() {}

@Suppress("EXPOSED_PARAMETER_TYPE")
fun suppressed(s: Secret) {}

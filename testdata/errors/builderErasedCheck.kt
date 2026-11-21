// x is C<A> for an x of a builder's variable that nothing fixed can't
// check C's arguments, as in K2, where x's type is the stub, though the
// variable's type, inferred later, would give them. A bare C fixes the
// variable for its arguments.
class Owner<T> {
    fun constrain(t: T) {}
    fun provide(): T = null!!
}

fun <OT> pcla(lambda: (Owner<OT>) -> Unit): OT = null!!

interface Base
open class Generic<A> : Base
open class GenericSub<B> : Generic<B>()
interface Arg
object Interloper : Base

fun withArguments() {
    pcla { o ->
        o.constrain(Generic<Arg>())
        o.provide() is GenericSub<Arg> // ERROR cannot check for instance of erased type: GenericSub<Arg>
        o.constrain(Interloper)
    }
}

fun bare() {
    pcla { o ->
        o.constrain(Generic<Arg>())
        o.provide() is GenericSub
        o.constrain(Interloper) // ERROR type mismatch
    }
}

fun stars() {
    pcla { o ->
        o.constrain(Generic<Arg>())
        o.provide() is GenericSub<*>
        o.constrain(Interloper)
    }
}

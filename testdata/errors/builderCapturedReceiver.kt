// The value of a member whose type is a type parameter that the
// receiver's type projects doesn't fix a builder's variable for its own
// members, as K2's captured type doesn't (KT-71843): the variable is
// what the calls after give it.
class Owner<T, CT : T> {
    fun constrain(t: T) {}
    fun provide(): CT = null!!
}

fun <OT> pcla(lambda: (Owner<OT, out OT>) -> Unit): OT = null!!

interface Base
class ScopeOwner : Base {
    fun function() {}
}
object Interloper : Base

fun test() {
    pcla { o ->
        o.constrain(ScopeOwner())
        o.provide().function() // ERROR unresolved reference: function
        o.constrain(Interloper)
    }
}

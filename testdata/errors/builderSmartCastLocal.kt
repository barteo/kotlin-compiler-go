// A member of a local of a builder's type that a type check narrowed to
// an unrelated type: the receiver is still of the builder's variable,
// which the member fixes, and it has the members of both, kotlinc's
// Marker & Scope. A value that isn't a local isn't smart-cast, and
// its member isn't found (KT-71867).
class Owner<T> {
    fun constrain(value: T) {}
    fun provide(): T = null!!
}

fun <OT> pcla(lambda: (Owner<OT>) -> Unit): OT = null!!

interface Base

open class Scope : Base {
    fun function() {}
}

interface Marker {
    fun mark() {}
}

object Other : Base

fun local() {
    pcla { owner ->
        owner.constrain(Scope())
        val value = owner.provide()
        if (value is Marker) {
            value.function()
            value.mark()
        }
        owner.constrain(Other) // ERROR type mismatch
    }
}

fun call(): Base = pcla { owner ->
    owner.constrain(Scope())
    if (owner === Owner<Marker>()) {
        owner.provide().function() // ERROR unresolved reference: function
    }
    owner.constrain(Other)
}

// A member declared as its class's CT, of a receiver whose argument for
// CT is projected, is of a captured type in kotlinc: unlike a builder's
// variable itself, it isn't fixed for its members, and it has none
// (KT-71843, KT-71844, KT-71845). The variable stays open: Other fits it
// afterwards.
class Owner<T, CT> {
    fun constrain(value: T) {}
    fun provide(): CT = null!!
    val provided: CT = null!!
}

class Bounded<T, CT : T> {
    fun constrain(value: T) {}
    fun provide(): CT = null!!
}

fun <OT> out(lambda: (Owner<OT, out OT>) -> Unit): OT = null!!
fun <OT> contra(lambda: (Bounded<OT, in OT>) -> Unit): OT = null!!
fun <OT> star(lambda: (Bounded<OT, *>) -> Unit): OT = null!!
fun <OT> inv(lambda: (Owner<OT, OT>) -> Unit): OT = null!!

interface Base

class Scope : Base {
    fun function() {}
    val property = 1
}

object Other : Base

fun captured() {
    val a: Base = out { owner ->
        owner.constrain(Scope())
        owner.provide().function() // ERROR unresolved reference: function
        owner.constrain(Other)
    }
    val b: Base = out { owner ->
        owner.constrain(Scope())
        owner.provided.property // ERROR unresolved reference: property
        owner.constrain(Other)
    }
    val c: Base = contra { owner ->
        owner.constrain(Scope())
        owner.provide().function() // ERROR unresolved reference: function
        owner.constrain(Other)
    }
    val d: Base = star { owner ->
        owner.constrain(Scope())
        owner.provide().function() // ERROR unresolved reference: function
        owner.constrain(Other)
    }
}

// Without a projection it is the variable, which the call fixes.
fun fixed() {
    inv { owner ->
        owner.constrain(Scope())
        owner.provide().function()
        owner.constrain(Other) // ERROR type mismatch
    }
}

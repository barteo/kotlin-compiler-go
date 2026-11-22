// A member of a local declared with a builder's type, after an
// assignment: the receiver's type is still the builder's variable, which
// the member fixes to what was assigned so far (KT-63841). An assignment
// of another type after it is a mismatch, and the local keeps the type.
class Target { fun member() {} }
class Different

class Buildee<TV> {
    fun get(): TV = null!!
}

fun <PTV> build(instructions: Buildee<PTV>.() -> Unit): Buildee<PTV> = Buildee()

fun fixes(): Buildee<Target> = build {
    var v = get()
    v = Target()
    v.member()
}

fun unresolved() {
    build {
        var v = get()
        v = Different()
        v.member() // ERROR unresolved reference: member
    }
}

fun mismatch() {
    val b = build {
        var v = get()
        v = Target()
        v.member()
        v = Different() // ERROR type mismatch
        v.member()
        v = Target()
        v.member()
    }
    val t: Buildee<Target> = b
}

// Without a member in between, both assignments give the type.
fun open(): Buildee<Any> = build {
    var v = get()
    v = Target()
    v = Different()
}

class Sum { operator fun plus(o: Sum) = this }

// An operator's receiver, as a member's.
fun operator() {
    build {
        var v = get()
        v = Sum()
        v + Sum()
        v = Different() // ERROR type mismatch
    }
}

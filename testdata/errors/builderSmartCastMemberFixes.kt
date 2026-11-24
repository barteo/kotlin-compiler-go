// A member call on a variable declared with a builder's type variable and
// smart-cast by an assignment fixes the variable, as kotlinc does: the
// later assignment of another type is a mismatch.
class TargetType { fun member() {} }
class DifferentType
class Buildee<TV> { fun get(): TV = null!!; fun set(v: TV) {} }
fun <PTV> build(instructions: Buildee<PTV>.() -> Unit): Buildee<PTV> = Buildee()

fun memberCall() {
    val a = build {
        var v = get()
        v = TargetType()
        v.member()
        v = DifferentType() // ERROR type mismatch: inferred type is DifferentType but TargetType was expected
    }
    val a2: Int = a // ERROR type mismatch: inferred type is Buildee<TargetType> but Int was expected
}

fun anyMember() {
    val d = build {
        var v = get()
        v = TargetType()
        v.toString()
        v = DifferentType() // ERROR type mismatch: inferred type is DifferentType but TargetType was expected
    }
    val d2: Int = d // ERROR type mismatch: inferred type is Buildee<TargetType> but Int was expected
}

fun noMember() {
    val c = build {
        var v = get()
        v = TargetType()
        set(DifferentType())
    }
    val c2: Int = c // ERROR type mismatch: inferred type is Buildee<Any> but Int was expected
}

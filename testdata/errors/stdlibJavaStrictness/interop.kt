// Java declarations kotlinc reads more strictly than their Kotlin look.
// The marked lines are kotlinc's with annotations-13.0.jar on the class
// path, as its own tests have, for J's @Nullable.

fun alwaysNull() {
    val my = My.create()
    if (my == null) {
        my.foo() // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed
    }
    if (my != null) {
        my.foo()
    }
}

class Derived : My() {
    fun foo2() {
        if (field == null) {
            field.length
        }
        if (finalField == null) {
            finalField.length // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed
        }
    }
}

fun throwNotNull() {
    throw J.staticNN
}

fun throwNullable() {
    throw J.staticN // ERROR type mismatch
}

fun throwPlatform() {
    throw J.staticJ
}

fun sealedEmpty(base: Base) = when (base) { // ERROR 'when' expression must be exhaustive
}

fun sealedAll(base: Base) {
    when (base) {
        is A -> {}
        is B -> {}
    }
    when (Base.provide()) {
        is A -> {}
        is B -> {}
    }
}

fun sealedPlatform() = when (Base.provide()) { // ERROR 'when' expression must be exhaustive
}

class Bar : Foo() // ERROR needs opt-in to inherit

@OptIn(ExperimentalMultiplatform::class)
class OptedIn : Foo()

@SubclassOptInRequired(ExperimentalMultiplatform::class)
open class Propagated : Foo()

class Recursive : Rec() {
    override fun getFoo() = foo // ERROR recursive problem
    override fun getBar(): Int = bar
}

// JB's JA<String!> gives way to the program's own JA<String?>.
class KNullable : JB(), JA<String?>
class KNotNull : JB(), JA<String>
interface KX : JA<String>
class KThrough : JB(), KX

fun eatJA(a: JA<String>) {}
fun eatJAN(a: JA<String?>) {}

fun supertypes(b: JB, n: KNullable, nn: KNotNull, t: KThrough) {
    eatJA(b)
    eatJAN(b)
    eatJA(n) // ERROR type mismatch
    eatJAN(n)
    eatJA(nn)
    eatJAN(nn) // ERROR type mismatch
    eatJA(t)
    eatJAN(t) // ERROR type mismatch
}

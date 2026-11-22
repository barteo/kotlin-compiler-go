class TargetType
class Buildee<TV> { fun get(): TV = null!!; fun set(v: TV) {} }
fun <PTV> build(instructions: Buildee<PTV>.() -> Unit): Buildee<PTV> = Buildee()
fun <PTV> build2(block: PTV.() -> Unit): PTV = null!!
fun <T> id(x: T): T = x
fun plain(b: Buildee<TargetType>) {}
fun plainAny(b: Any) {}
fun consume(value: TargetType) {}

// As kotlinc, a generic call can't infer its type parameter from a value
// whose type has a type argument that nothing inferred.
fun test() {
    val buildee = build { } // ERROR cannot infer type for type parameter 'PTV'
    id<Buildee<TargetType>>(buildee) // ERROR cannot infer type for type parameter 'T'
    id(buildee) // ERROR cannot infer type for type parameter 'T'
    plain(buildee)
    plainAny(buildee)
    buildee.get()
    val l = listOf(buildee) // ERROR cannot infer type for type parameter 'T'
    id<List<Buildee<TargetType>>>(l) // ERROR cannot infer type for type parameter 'T'
}

// And this of a lambda whose receiver's type wasn't inferred is reported.
fun test2() {
    val t = build2 { // ERROR cannot infer type for type parameter 'PTV'
        consume(this) // ERROR cannot infer type for type parameter 'PTV'
    }
    id<TargetType>(t)
    val z = build2 { // ERROR cannot infer type for type parameter 'PTV'
        val q: TargetType = this // ERROR cannot infer type for type parameter 'PTV'
    }
}

class DifferentType
fun <T : DifferentType> consumeDifferent(value: T) {}

// As kotlinc, a call in the lambda whose type parameter's bound the
// builder's constraints contradict is that call's error: the builder's
// type argument is inferred from the others.
fun test3() {
    val buildee = build {
        set(TargetType())
        consumeDifferent(get()) // ERROR is not a subtype of DifferentType
    }
    plain(buildee)
}

// Builder inference as K2's PCLA does it: a nested call's lambda fixes
// the builder's variable it takes as a parameter, an anonymous object's
// member and an @Exact parameter don't constrain the variables, a
// lambda whose receiver needs the builder can't choose an overload, and
// the capture of an out projection of the builder's variable can't be
// inferred. A nullable variable below the builder's makes it nullable.

import kotlin.experimental.ExperimentalTypeInference

fun lambdaInput(x: List<String>) = buildList {
    add("")
    addAll(flatMap { listOf(2) }) // ERROR none of the 3 candidates
    addAll(flatMap { x })
}

class TargetType

interface TypeSource {
    fun produce(): TargetType
}

class Buildee<TV> {
    fun get(): TV = null!!
    fun getOut(): Inv<out TV> = Inv()
    fun getIn(): In<TV> = In()
    fun emit(v: TV) {}
}

class Inv<T>
class In<in T>

fun <PTV> build(instructions: Buildee<PTV>.() -> Unit): Buildee<PTV> = Buildee<PTV>().apply(instructions)

fun anonymousObject() {
    build { // ERROR cannot infer type for type parameter 'PTV'
        object : TypeSource {
            override fun produce() = get()
        }
    }
}

fun <K> capture(x: Inv<K>): K = null!!

fun capturedOut() {
    build {
        emit("")
        capture(getOut()) // ERROR cannot infer type for type parameter 'K'
    }
}

fun <E> nullableBelow(x: E, y: In<E?>): E = x

fun nullable() {
    val b: Buildee<String?> = build {
        emit("")
        nullableBelow(get(), getIn())
    }
}

@OptIn(ExperimentalTypeInference::class)
@OverloadResolutionByLambdaReturnType
fun <E1> buildL(x: MutableList<E1>.() -> Int): Int = 1

@OptIn(ExperimentalTypeInference::class)
@OverloadResolutionByLambdaReturnType
fun <E2> buildL(x: MutableList<E2>.() -> String): String = ""

fun byLambdaResult() {
    buildL { // ERROR ambiguous call
        ""
    }
}

@Suppress("INVISIBLE_MEMBER", "INVISIBLE_REFERENCE")
fun <T> exactly(e: @kotlin.internal.Exact T) {}

fun exact(b: Buildee<String>, c: Buildee<*>) {
    exactly<Buildee<*>>(c)
    exactly<Buildee<*>>(b) // ERROR type mismatch: inferred type is Buildee<String> but Buildee<*> was expected
}

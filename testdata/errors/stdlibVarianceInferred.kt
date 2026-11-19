// Inferred types, receivers and member type parameters' bounds break a
// class's variance as written ones do.
import kotlin.reflect.KProperty

interface In<in T>

class Delegate<T> {
    operator fun getValue(t: Any, p: KProperty<*>): T = null!!
}

fun <T> getT(): T = null!!

abstract class Test<in I, out O, P> {
    val implicitType1 = getT<I>() // ERROR type parameter I is declared as 'in' but occurs in 'out' position
    val implicitType2 = getT<O>()
    val delegateType1 by Delegate<I>() // ERROR type parameter I is declared as 'in' but occurs in 'out' position
    val delegateType2 by Delegate<O>()
    private val p = getT<I>()

    abstract val I.receiver1: Int
    abstract val O.receiver2: Int // ERROR type parameter O is declared as 'out' but occurs in 'in' position
    fun <X : I> bounded1() {}
    fun <X : O> bounded2() {} // ERROR type parameter O is declared as 'out' but occurs in 'in' position
    fun inferred1() = getT<O>()
    fun inferred2() = getT<I>() // ERROR type parameter I is declared as 'in' but occurs in 'out' position
}

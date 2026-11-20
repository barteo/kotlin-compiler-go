// Errors of callable references whose types kc infers.

import Obj.len

class B {
    fun B.ext(): Int = 1
    val B.prop: Int get() = 2

    fun test(b: B) {
        val r = B::ext // ERROR a reference to an extension declared in a class is not allowed
        val f: (B) -> Int = B::ext // ERROR a reference to an extension declared in a class is not allowed
        val p = B::prop // ERROR a reference to an extension declared in a class is not allowed
        val q = b::prop // ERROR a reference to an extension declared in a class is not allowed
    }
}

object Obj {
    val String.len: Int get() = length
}

fun obj() = String::len // ERROR a reference to an extension declared in a class is not allowed

fun top() {}

val v = ::top == 1 // ERROR operator '==' cannot be applied

// As kotlinc, a reference expected to return Unit infers the referenced
// function's type parameter from the expected result too, which a
// result of that type parameter its arguments give another type can't
// take; passed for a parameter of a function type, it is adapted to drop
// the result.
fun <T> unitId(x: T): T = x
fun <T> unitIdOrNull(x: T): T? = x
fun <T> unitList(x: T): Array<T> = null!!
fun <T> unitNoArgs(): T = null!!
fun <N : Number> unitNumber(n: N) = n
fun takeUnit(f: (Int) -> Unit) {}
fun unitRefs(b: Boolean): (Int) -> Unit {
    val r1: (Int) -> Unit = ::unitId // ERROR cannot infer type for type parameter 'T'
    val r2: (Int) -> Unit = ::unitIdOrNull // ERROR cannot infer type for type parameter 'T'
    val r3: (Int) -> Unit = if (b) ::unitId else r1 // ERROR cannot infer type for type parameter 'T'
    val r4: (Int) -> Unit = ::unitList
    val r5: () -> Unit = ::unitNoArgs
    val r6: (Int, Any) -> Any = ::unitId2
    takeUnit(::unitId)
    42.unitApply(::unitNumber)
    return ::unitId // ERROR cannot infer type for type parameter 'T'
}
fun <T> unitId2(x: T, y: Any): T = x
fun <T> T.unitApply(f: T.() -> Unit): T = this

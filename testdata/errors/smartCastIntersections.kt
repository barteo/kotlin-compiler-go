// Smart casts as K2's intersections with the declared type.

open class B
interface A
interface C

fun gav(b: B): A? = if (b is A && b is C) b else null
fun cee(b: B): C? = if (b is A && b is C) b else null
fun dee(b: B): String? = if (b is A && b is C) b else null // ERROR type mismatch

class Box<V>(val value: V)

fun <T, V> same(t: Box<out T>, v: Box<out V>) {
    if (t == v) {
        val x: T = v.value
        val y: V = t.value
    }
    val z: V = t.value // ERROR type mismatch
}

interface Slice<X>
interface Super
interface SubA : Super
interface SubB : Super
fun <Y> unbox(s: Slice<out Y>): Y? = null

fun <P> either(p: Slice<P>, a: Slice<SubA>, b: Slice<SubB>) {
    if (a === p || b === p) {
        unbox<Super>(p)
        unbox<P>(p)
        unbox<SubA>(p) // ERROR type mismatch
    }
}

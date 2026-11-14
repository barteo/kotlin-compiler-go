// Captured types: a projected argument is captured for inference and
// approximated where the result escapes.
class Box<T>(var v: T) {
    fun get(): T = v
    fun set(x: T) { v = x }
    fun each(f: (T) -> Unit) { f(v) }
}

class Inv<T>(val v: T)

fun <T> id(b: Box<T>): Box<T> = b
fun <T> first(b: Box<T>): T = b.get()
fun <T> nested(b: Box<Box<T>>): T = b.get().get()
fun <T> Box<T>.ext(f: (T) -> Unit) { f(get()) }
fun <T> Box<T>.pair(f: (T, T) -> Unit, g: (T, T) -> Unit) { f(get(), get()) ; g(get(), get()) }

fun number(n: Number): Int = n.toInt()

fun outBox(b: Box<out Number>) {
    val n: Number = first(b)
    val c: Box<out Number> = id(b)
    number(b.get())
    b.each { number(it) }
    b.ext { number(it) }
    b.pair({ x, y -> number(x) + number(y) }) { u, w -> number(u) + number(w) }
    b.set(1) // ERROR type mismatch
    val bad: Box<Number> = id(b) // ERROR type mismatch
}

fun inBox(b: Box<in Int>) {
    b.set(1)
    val a: Any? = first(b)
    b.each { val x: Any? = it }
    val bad: Int = first(b) // ERROR type mismatch
}

fun nestedBox(b: Box<Box<out Number>>) {
    val n: Number = nested(b) // ERROR type mismatch
}

fun arrays(a: Array<out Inv<Int>>) {
    val i: Inv<Int> = a[0]
    val b: Inv<out Int> = first(Box(a[0]))
}

// A common supertype within the expected type's bound.
interface RO<out E>
class Set2<E> : RO<E>
class My<T>
interface A
interface D
class B : A, D
class C : A, D
fun <T> setOf2(x: T, y: T): Set2<T> = Set2()
fun expected(b: My<B>, c: My<C>) {
    val s: RO<My<out D>> = setOf2(b, c)
    val t: RO<My<B>> = setOf2(b, c) // ERROR type mismatch
}

// T & Any read through a star projection.
class Res<out T>
interface Conv<T> {
    fun conv(): Res<T & Any>
}
fun Conv<*>.cnv(): Res<Any> = conv()

// A recursive bound read through a star projection.
interface Rec<T : Rec<T?>?> {
    fun foo(): T?
}
fun rec(r: Rec<*>): Rec<*>? = r.foo()

// A type alias's arguments are not approximated.
typealias Arr2<T> = Array<Array<T>>
fun arr2(a: Arr2<out Number>) = a
fun arr2Int(a: Arr2<Int>) = arr2(a) // ERROR type mismatch

// Below a captured parameter is only Nothing.
interface SelfFoo<S : SelfFoo<S>> {
    fun <T : S> bar(t: T)
}
fun selfStar(f1: SelfFoo<*>, f2: SelfFoo<*>) {
    f1.bar(f2) // ERROR not a subtype
}

// A star in an invariant argument read is a capture approximated.
interface Two<R> {
    fun boxed(): Box<Box<R>>
}
fun two(t: Two<*>) {
    val a: Box<out Box<*>> = t.boxed()
    val b: Box<Box<*>> = t.boxed() // ERROR type mismatch
}

// Annotation classes whose instances another module creates, omitting
// arguments: their default values are the class file's AnnotationDefault.
package a

import kotlin.reflect.KClass

enum class E { X, Y }

annotation class Two<K, V>(val s: String)

annotation class Nesting(val a2: Two<String, List<String>> = Two("two"))

annotation class WithKClass<K : Any>(val k: KClass<K>)

annotation class All(
    val i: Int = 1,
    val l: Long = 2,
    val u: UInt = 3u,
    val big: ULong = ULong.MAX_VALUE,
    val s: String = "s",
    val e: E = E.Y,
    val k: KClass<*> = String::class,
    val ki: KClass<*> = Int::class,
    val arr: IntArray = [1, 2],
    val ks: Array<KClass<*>> = [Int::class, List::class],
    val es: Array<E> = [E.X],
    val n: Array<Two<Int, Int>> = [Two("a"), Two("b")],
    val b: Boolean = true,
    val c: Char = 'c',
    val d: Double = 1.5,
    val f: Float = 2.5f,
    val empty: Array<String> = [],
)

annotation class Outer(val inner: Array<Inner> = [Inner([1]), Inner([])]) {
    annotation class Inner(val v: IntArray = [], val tag: String = "t")
}

// Generic signatures of unboxed generic value classes: their underlying
// types with the classes' type arguments, or erased if that is the
// class's type parameter (TestSignatures).
package p

@JvmInline value class VA<T>(val x: T)
@JvmInline value class VB<T>(val x: List<T>)
@JvmInline value class VI<T : Int>(val s: T)

fun vf(a: VA<String>) = a.x
fun vg(b: VB<String>) = b.x
fun <U> vh(b: VB<U>, a: VA<U>) = b.x
fun vj(): VB<Int> = VB(listOf(1))
fun vk(w: VI<Int>) = w.s
fun vn(b: VB<String>?) = b?.x
@JvmInline value class VN<T : Number>(val n: T)
@JvmInline value class VP<T>(val n: Int)
@JvmInline value class VM<K, V : Comparable<V>>(val m: Map<K, V>)

fun vnum(n: VN<Long>, p: VP<String>, m: VM<String, Int>) = n.n.toInt() + p.n + m.m.size

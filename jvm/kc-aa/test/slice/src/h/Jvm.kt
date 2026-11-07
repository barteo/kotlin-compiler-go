package h

@JvmInline value class Id(val v: Long)
@JvmInline value class Name(val s: String)

class Sigs<T : CharSequence, U>(val id: Id, val name: Name?) {
    fun prims(a: Int, b: Int?, c: Boolean, d: Char, e: Double): Long = 0
    fun arrays(a: IntArray, b: Array<Int>, c: Array<String?>, d: Array<in String>, e: Array<out Number>, f: Array<IntArray>): Array<Array<T>>? = null
    fun generics(t: T, u: U, l: List<T>, m: MutableMap<String, U>, e: Map.Entry<T, U>): T? = null
    fun builtins(a: Any, s: String, c: Comparable<Int>, n: Number, th: Throwable, cs: CharSequence, i: Inner) {}
    fun functions(f: (Int) -> String, g: suspend (String) -> Unit, h: Int.() -> Unit): Nothing = TODO()
    fun values(a: Id, b: Id?, c: Name?, d: List<Id>, e: UInt, f: kotlin.time.Duration?, g: kotlin.time.Duration): Id = a
    val nothing: Nothing? get() = null
    val star: List<*> get() = listOf<Any>()

    class Inner
}

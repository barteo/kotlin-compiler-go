@JvmInline
value class Code(val code: Int) : Comparable<Code> {
    override fun compareTo(other: Code): Int = code - other.code
    override fun toString(): String = "#" + code
}

@JvmInline
value class Opt(val s: String?) {
    fun orEmpty(): String = if (s == null) "" else s
}

@JvmInline
value class Big(val v: Long) {
    fun add(o: Big): Big = Big(v + o.v)
}

fun <T : Comparable<T>> maxOf2(a: T, b: T): T = if (a > b) a else b

fun pick(x: Any?): String = when (x) {
    is Code -> "code " + x.code
    is Opt -> "opt " + x.orEmpty()
    null -> "null"
    else -> "other"
}

fun apply(f: (Code) -> Code, c: Code): Code = f(c)

fun main() {
    println(Code(1) < Code(2))
    println(maxOf2(Code(3), Code(2)))
    println(pick(Code(4)))
    println(pick(Opt(null)))
    println(pick(Opt("x")))
    println(pick(null))
    println(Opt(null).orEmpty().length)
    val o: Opt? = null
    println(o)
    var c: Code? = null
    println(c)
    c = Code(5)
    println(c)
    val m = java.util.HashMap<Code, String>()
    m.put(Code(1), "one")
    println(m.get(Code(1)))
    println(apply({ x -> Code(x.code + 1) }, Code(1)))
    var total = Code(0)
    val add = { k: Int -> total = Code(total.code + k) }
    add(3)
    add(4)
    println(total)
    println(Big(1L).add(Big(2L)))
    val arr = arrayOf(Code(1), Code(2))
    println(arr[1])
    arr[0] = Code(9)
    println(arr[0].code)
    for (x in arr) {
        println(x.code)
    }
}

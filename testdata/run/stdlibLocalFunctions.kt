fun scores(names: List<String>): List<String> {
    var calls = 0
    fun score(s: String): Int {
        calls++
        return s.length * 10
    }
    val out = names.map { name -> name + "=" + score(name) }
    println("calls: " + calls)
    return out
}

fun firstLong(names: List<String>): String {
    fun isLong(s: String) = s.length > 3
    names.forEach {
        if (isLong(it)) {
            return it
        }
    }
    return "none"
}

fun refs(xs: List<Int>): List<Int> {
    val offset = 100
    fun shift(x: Int): Int = x + offset
    return xs.map(::shift)
}

fun builder(): String = buildString {
    fun line(s: String) {
        append(s).append('/')
    }
    line("a")
    line("b")
}

fun insideLambda(xs: List<Int>): Int {
    var sum = 0
    xs.forEach { x ->
        fun add(k: Int) {
            sum += k * x
        }
        add(1)
        add(2)
    }
    return sum
}

fun recursiveInLambda(n: Int): List<Int> {
    val out = mutableListOf<Int>()
    fun walk(k: Int) {
        out.add(k)
        if (k > 0) {
            listOf(k - 1).forEach { walk(it) }
            val later = { walk(-1) }
            if (k == n) {
                later()
            }
        }
    }
    walk(n)
    return out
}

fun labeled(xs: List<Int>): Int {
    fun firstEven(): Int {
        xs.forEach {
            if (it % 2 == 0) {
                return@firstEven it
            }
        }
        return -1
    }
    return firstEven()
}

val lazyValue = run {
    fun compute(x: Int) = x * x
    compute(7)
}

fun main() {
    println(scores(listOf("ab", "abc")))
    println(firstLong(listOf("x", "long", "longer")))
    println(firstLong(listOf("x")))
    println(refs(listOf(1, 2)))
    println(builder())
    println(insideLambda(listOf(1, 2, 3)))
    println(recursiveInLambda(2))
    println(labeled(listOf(1, 3, 4, 6)))
    println(lazyValue)
}

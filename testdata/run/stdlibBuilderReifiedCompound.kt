inline fun <reified T> nameOf(x: T): String = T::class.simpleName ?: "?"
inline fun <reified T> List<*>.count2(): Int = count { it is T }

fun main() {
    val l = buildList {
        add("a")
        val arr = arrayOf(get(0), get(0))
        println(arr.javaClass.simpleName)
        println(listOf(get(0), 1).filterIsInstance<String>())
        println(count2<String>())
        var y = get(0)
        y += "z"
        add(y)
        var n = size
        n += get(0).length
        println(n)
    }
    println(l)
    val m = buildList {
        add(1)
        val arr = arrayOf(get(0))
        println(arr.javaClass.simpleName)
        var k = get(0)
        k *= 3
        k -= 1
        add(k)
    }
    println(m)
}

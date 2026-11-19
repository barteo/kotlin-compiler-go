package fixture

// Variables, collections and smart casts.

fun unusedLocal(): Int {
    val unused = 1
    return 2
}

fun varCouldBeVal(): Int {
    var x = 1
    return x
}

fun doubleMutability(): List<Int> {
    var list = mutableListOf(1)
    list.add(2)
    return list
}

fun smartCast(x: Any): Int {
    if (x is String) {
        return (x as String).length
    }
    return 0
}

fun lambdaParams(m: Map<String, Int>): Int = m.entries.sumOf { (k, v) -> v }

class Holder {
    private val unusedProperty = 1
    private fun unusedFunction() = 1
    fun used(): Int = 2
}

fun sorted(list: List<Int>): Int? = list.sorted().firstOrNull()

fun filterFirst(list: List<Int>): Int? = list.filter { it > 1 }.first()

fun mapNotNull(list: List<Int?>): List<Int> = list.map { it }.filterNotNull()

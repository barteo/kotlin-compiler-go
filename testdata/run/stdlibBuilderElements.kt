// The elements of a standard library builder, read back inside it, have
// the type its other calls give: a Java member's E!, as MutableList's
// get(0), and an operand of a built-in operator, fix it as a member does.

fun main() {
    println(buildList { add("x"); add(get(0).uppercase()); add(get(0) + "y") })
    println(buildList { add(1); add(-get(0)); add(get(0) * 10); add(1 + get(0)) })
    println(buildList { add("a"); add("b" + get(0)); add(this[0].length.toString()) })
    println(buildList { add(1); var y = get(0); y += 41; add(y) })
    println(buildList { add(3); println(get(0) < 5); println(get(0) == 3) })
    println(buildMap { put("a", 1); put("b", getValue("a") + 1); put("c", get("b")!! * 3) })
    println(buildMap<String, Int> { put("n", 1); getOrPut("m") { 2 } })
    println(buildSet { add(2); add(first() * 2) })
    println(buildList { add(listOf(1, 2)); add(get(0) + 3) })
    println(buildList { add(1L); add(get(0) + 1) })
    println(buildList { add('a'); add(get(0) + 1) })
    println(buildList { add(1.5); add(get(0) / 3) })
}

import reif.*

fun main() {
    val xs = sequenceOf<Any?>(1, "x", 2L, null, "y", 3)
    println(xs.only<String>())
    println(xs.only<Int>())
    println(xs.toList().firstOf<Long>())
    println(xs.toList().firstOf<Double>())
    println(xs.orNulls<String>())
    println(xs.only<String?>())
    println(xs.count2<Int>())
    println(xs.andStrings<Number>())
    println(picker<Int>().pick(xs))
    println(picker<String>().pick(xs))
}

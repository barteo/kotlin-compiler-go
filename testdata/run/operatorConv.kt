// Operator conventions on built-in and type parameter operands: operator
// extensions of String, Boolean and Int, inc with a default argument, and
// compound assignments of a type parameter's value with a numeric bound.

operator fun String.unaryPlus(): String = if (this == "") "done" else +""
operator fun String.invoke(i: Int) = "$this$i"
operator fun Boolean.unaryMinus() = if (this) "yes" else "no"

fun <R> block(f: () -> R): R = f()
fun <R> call(f: () -> R): R = f()

class Collector {
    val sb = StringBuilder()
    operator fun String.unaryPlus() {
        sb.append(this)
    }
    operator fun Int.invoke(other: Int) = this + other
    operator fun String.invoke() = "invoked $this"

    fun collect(): String {
        +"a"
        +"b"
        return sb.toString() + " " + (1200)(34) + " " + block { "abc" }()
    }
}

object Hidden {
    private operator fun String.unaryMinus() = "($this)"
    fun wrap() = call { -"x" }
}

var result = "Fail"

class A

operator fun A.inc(s: String = "OK"): A {
    result = s
    return this
}

var counter = 0
var total = 0L

class Counted<T : Int>(val i: T) {
    init {
        counter += i
        total += i
        counter *= i
    }
}

fun main() {
    println(+"11")
    println("a"(12))
    println(-true)
    println(Collector().collect())
    println(Hidden.wrap())
    val sb = StringBuilder()
    operator fun String.unaryMinus() {
        sb.append(this)
    }
    -"local"
    println(sb)
    var a = A()
    a++
    println(result)
    result = "Fail"
    ++a
    println(result)
    Counted(3)
    println("$counter $total")
    println(-5 + +3)
    println(!false)
}

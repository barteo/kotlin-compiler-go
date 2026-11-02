class A

operator fun A.inc(s: String = "OK"): A = this
operator fun A.dec(s: String): A = this // ERROR must have no value parameters
operator fun A.unaryPlus(s: String = ""): A = this // ERROR must have no value parameters
operator fun A.unaryMinus(vararg s: String): A = this // ERROR must have no value parameters

fun String.unaryMinus() = "not an operator"
operator fun String.invoke(i: Int) = "$this$i"

class Bounded<T : Int>(val i: T) {
    var s = ""
    var n = 0
    fun f() {
        n += i
        s -= i // ERROR operator '-=' cannot be applied to String and Int
    }
}

fun main() {
    println(+"x") // ERROR operator '+' cannot be applied to String
    println(-"x") // ERROR operator '-' cannot be applied to String
    println("a"("b")) // ERROR cannot be invoked as a function
    println(!"x") // ERROR type mismatch
}

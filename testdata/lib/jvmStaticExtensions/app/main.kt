import lib.Counter
import lib.Counter.Companion.even
import lib.Counter.Companion.times3
import lib.Texts
import lib.Texts.vowels
import lib.Texts.wrap

fun main() {
    println("a".wrap())
    println("b".wrap("<", ">"))
    println("education".vowels)
    with(Texts) {
        val none: String? = null
        println(none.orEmpty2() + "|" + "x".orEmpty2())
        val sb = StringBuilder("abc")
        sb.tail = 'q'
        sb.tail++
        println(sb.toString() + " " + sb.tail)
        val maybe: String? = "io"
        println(maybe?.vowels)
    }
    println(Texts.table["two"])
    Texts.mode = "b"
    Texts.mode += "c"
    println(Texts.mode)
    println(7.times3())
    println(4.even)
    println(Counter.start)
    with(Counter) {
        println(2.times3())
    }
}

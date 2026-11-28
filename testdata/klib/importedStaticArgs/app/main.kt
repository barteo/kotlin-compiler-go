import j.Names.first
import j.Util.name
import j.Util.next

class Label(val text: String) {
    companion object {
        operator fun invoke(n: Int): Label = Label("#$n")
    }
}

class Count(val n: Int) {
    companion object {
        operator fun invoke(s: String): Count = Count(s.length)
    }
}

fun main() {
    println(Label(name()).text)
    println(Label(next(1)).text)
    println(Label(first()).text)
    println(Count(next(2)).n)
    println(Count(name()).n)
}

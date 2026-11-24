// A class Array of the program's shadows kotlin.Array, but a call its
// constructors don't take is kotlin.Array's: Array(n) { init }.
class Array<E>(e: E) {
    val k = Array(2) { e as Any? }
}

// x op= v through a projected receiver calls plusAssign on the value
// read, which writes nothing: no Nothing setter is involved.
interface Tr<T> {
    var v: T
}

class Counter {
    var n = 0
    operator fun plusAssign(k: Int) {
        n += k
    }
}

class TrImpl(override var v: Counter) : Tr<Counter>

fun bump(t: Tr<out Counter>) {
    t.v += 5
}

fun main() {
    val a = Array("x")
    println(a.k.size.toString() + a.k[1])
    val t = TrImpl(Counter())
    bump(t)
    println(t.v.n)
}

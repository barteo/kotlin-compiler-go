// As in K2, x op= v is ambiguous only if both x.plusAssign(v) and
// x = x.plus(v) apply, and the second only if its result may be assigned
// to x: list - 2 is a List<Int>, not an ArrayList<Int>.
fun collections() {
    var l = ArrayList<Int>()
    l -= 2
    var m = mutableListOf(1)
    m += 2
    var s = mutableMapOf(1 to 1)
    s += 2 to 2
}

class MyInt(val i: Int) {
    operator fun plus(m: MyInt): MyInt = MyInt(m.i + i)
}

operator fun Any.plusAssign(a: Any) {}

fun builtins(m: MyInt) {
    var n = m
    n += m // ERROR assignment operators ambiguity
    var i = 1
    i += 34 // ERROR assignment operators ambiguity
    val j = 1
    j += 2
}

class C {
    operator fun get(i: Int): C = this
}

operator fun C.plus(a: String): C = this
operator fun C.plusAssign(a: String) {}

class C1 {
    operator fun get(i: Int): C = C()
    operator fun set(i: Int, v: C) {}
}

fun elements() {
    val c = C()
    c[0] += ""
    val c1 = C1()
    c1[0] += "" // ERROR assignment operators ambiguity
}

// Without an assignment operator extension in scope, as the one for Any
// in k.kt: x op= v takes the form that applies, and an element's
// plusAssign when set can't take the result of plus.
package v

fun readOnly() {
    var l = listOf(1)
    l += 2
    var s = "a"
    s += 1
    var n = 1
    n += 2
}

fun elements() {
    val a = arrayOf(1)
    a[0] += 1
    val ll = mutableListOf(mutableListOf(1))
    ll[0] += 2
    val m = mutableMapOf("a" to listOf(1))
    m["a"] = m.getValue("a") + 2
    val ml = mutableListOf(listOf(1))
    ml[0] += 2
}

class Counter(val i: Int) {
    operator fun plus(d: Int) = Counter(i + d)
}

class Box(var c: Counter)

fun properties(b: Box) {
    b.c += 1
}

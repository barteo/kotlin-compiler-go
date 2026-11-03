fun len(s: String?): Int = s.length // ERROR only safe (?.) or non-null asserted (!!.) calls are allowed

fun smart(s: String?, t: String?): Int {
    if (s != null) {
        println(s.length)
    }
    if (t == null) return 0
    var v: String? = "x"
    if (v != null) {
        println(v.length)
        v = null
        println(v.length) // ERROR only safe (?.) or non-null asserted
    }
    var w: String? = "x"
    val clear = { w = null }
    if (w != null) {
        clear()
        println(w.length) // ERROR only safe (?.) or non-null asserted
    }
    return t.length
}

fun arith(a: Int?): Int = a + 1 // ERROR operator '+' cannot be applied to Int? and Int

fun incNullable(a: Int?) {
    var x = a
    x++ // ERROR operator '++' cannot be applied to Int?
    x += 1 // ERROR operator '+=' cannot be applied to Int? and Int
    var s: String? = "a"
    s -= "b" // ERROR operator '-=' cannot be applied to String? and String
}

operator fun Int?.minus(s: String): Int = 0

fun nullableOperands(a: Int?, l: Long?) {
    println(a - "x") // Int?.minus(String)
    println(a - 1) // ERROR operator '-' cannot be applied to Int? and Int
    println(l < l) // ERROR operator '<' cannot be applied to Long? and Long?
}

fun recursive() = recursive() // ERROR recursive problem

// A var that a local function assigns, as kotlinc has it: in the function,
// unstable if the code after it assigns the var; in the code around it,
// unstable once the function exists if it assigns a value of another type.
fun captured(): Int {
    var x: String? = "z"
    fun reads(): Int {
        if (x != null) return x.length // ERROR only safe (?.) or non-null asserted
        return 0
    }
    x = "b"
    var y: String? = "z"
    fun clears() { y = null }
    if (y != null) return y.length // ERROR only safe (?.) or non-null asserted
    var z: String? = null
    fun sets() { z = "abc" }
    sets()
    if (z != null) return z.length + reads()
    return 0
}

// A loop keeps the smart cast of a var it only increments, decrements and
// assigns with op=, but not of one it also assigns another value, or that
// a function in it assigns.
operator fun Int.plus(s: String): Int? = if (s == "") null else this + s.length

fun loopSmartCasts(): Int {
    var i: Int? = 1
    if (i != null) {
        while (i != 10) {
            i++
            i -= 2
        }
        val n: Int = i
    }
    var j: Int? = 1
    if (j != null) {
        while (j != 10) {
            j++ // ERROR operator '++' cannot be applied to Int?
            j = null
        }
    }
    var k: Int? = 1
    if (k != null) {
        while (k != 10) {
            k-- // ERROR operator '--' cannot be applied to Int?
            fun clear() { k = null }
        }
    }
    var a: Any = 1
    if (a is Int) {
        do {
            a += 1 // ERROR operator '+=' cannot be applied to Any
        } while (a != 10)
    }
    var m: Int? = 1
    if (m != null) {
        while (m != 10) {
            m += "" // ERROR smart cast to 'Int' is impossible, because 'm' is a local variable that the loop changes to Int?
        }
    }
    return 0
}

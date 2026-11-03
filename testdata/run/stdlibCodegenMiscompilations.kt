// Programs that kc compiled and ran with wrong results.

// break and continue in a do-while loop's condition leave or repeat that
// loop, not the enclosing one.
fun breakInCondition(): String {
    var s = ""
    var i = 0
    while (i < 2) {
        i++
        do {
            s += "a"
        } while (if (s.length > 3) break else true)
        s += "b"
    }
    // continue goes to the condition again, not to the body.
    var n = 0
    do {
        s += "c"
    } while (if (n++ < 3) continue else false)
    return s + n
}

// T? with T := Unit is Unit?: { null } returns null, not Unit.
inline fun <T> inlineOrNull(f: () -> T?): T? = f()
fun <T> orNull(f: () -> T?): T? = f()

fun unitNull(): String {
    val a = inlineOrNull<Unit> { null }
    val b = orNull<Unit> { null }
    val c = inlineOrNull<Unit> { Unit }
    return "$a $b $c"
}

// A safe call of a Unit function is null if its receiver is.
class A {
    fun f() {}
}

val Any?.ext: String get() = toString()

fun safeUnit(a: A?): String = (a?.f()).ext + " " + a?.f().ext

fun main() {
    println(breakInCondition())
    println(unitNull())
    println(safeUnit(null))
    println(safeUnit(A()))
}

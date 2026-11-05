// Smart casts that flow through safe-call chains, ?: and casts, try and
// catch, loops, aliases, this@C, object expressions and open vals read
// through a final class.

class Box(val inner: Box?, val name: String?)

open class Base(open val v: Any?)

class Final(v: Any?) : Base(v)

abstract class Task(val arg: Int) {
    abstract fun run(): Int
}

class Holder(val s: String?) {
    fun len(): Int = if (this@Holder.s != null) s.length else -1
}

class Late {
    val s: String?

    init {
        s = "late"
        println(s.length) // s is the String assigned to it
    }
}

fun chain(b: Box?): Int {
    if (b?.inner?.name != null) {
        return b.inner.name.length // a?.b != null: a and a.b aren't null
    }
    return 0
}

fun asserted(b: Box?): Int {
    b?.inner!!
    return b.inner.hashCode() * 0 + 1 // a?.b!! means a isn't null
}

fun elvisTrue(b: Box?): String {
    if (b?.name?.equals("") ?: false) {
        return "empty " + b.name
    }
    return "other"
}

fun castOrReturn(x: Any): Int {
    (x as? String) ?: return -1
    return x.length
}

fun alias(name: String?): Int {
    when (val s = name) {
        null -> return 0
    }
    return name.length + 0
}

fun finalOpen(f: Final): Int {
    if (f.v is String) {
        return f.v.length // Final doesn't override v
    }
    return 0
}

fun tryCatch(p: String?): Int {
    var s: String? = null
    try {
        s = p!!
    } catch (e: NullPointerException) {
        return -1
    }
    return s.length
}

fun loop(p: String?): Int {
    var s: String? = p
    while (s == null || s.length < 3) {
        s = (s ?: "") + "x"
    }
    return s.length
}

fun shadow(p: Any?): Int {
    var x: Any? = p
    if (x is String) {
        for (i in 0..0) {
            val x = i // a different x
            println(x)
        }
        return x.length
    }
    return 0
}

fun objectArg(a: Int?): Int {
    val t = object : Task(a!!) {
        override fun run(): Int = arg
    }
    return t.run() + a
}

fun main() {
    println(chain(Box(Box(null, "abc"), null)))
    println(chain(Box(null, null)))
    println(asserted(Box(Box(null, null), null)))
    println(elvisTrue(Box(null, "")) + " " + elvisTrue(null))
    println(castOrReturn("four") + castOrReturn(4))
    println(alias("five") + alias(null))
    println(finalOpen(Final("six")) + finalOpen(Final(6)))
    println(Holder("hi").len() + Holder(null).len())
    println(Late().s)
    println(tryCatch("x") + tryCatch(null))
    println(loop(null) + loop("long"))
    println(shadow("abc") + shadow(1))
    println(objectArg(2))
}

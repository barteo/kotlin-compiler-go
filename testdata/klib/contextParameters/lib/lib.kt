// A library's declarations with context parameters, which its classes
// take first, before an extension receiver: functions, with default
// values, inline and suspend; properties, top-level, extension and
// member, and member extensions; and an abstract member a subclass
// overrides.
package lib

class Ctx(val n: String) {
    var log = ""
}

context(c: Ctx)
fun f(useArg: Boolean, arg: String = "arg") = if (useArg) arg else c.n

context(c: Ctx)
fun String.ext(): String = "$this ${c.n}"

context(c: Ctx)
inline fun <T> withLog(f: () -> T): T {
    c.log += "<"
    return f()
}

context(c: Ctx)
suspend fun fetch(id: Int): String = "fetched $id ${c.n}"

context(c: Ctx)
val top: String
    get() = "top ${c.n}"

context(c: Ctx)
var topVar: String
    get() = c.log
    set(v) {
        c.log += v
    }

context(c: Ctx)
val Int.extProp: String
    get() = "ext $this ${c.n}"

open class M(val m: String) {
    context(c: Ctx)
    val mem: String
        get() = "mem $m ${c.n}"

    context(c: Ctx)
    var memVar: String
        get() = c.log
        set(v) {
            c.log += "[$m$v]"
        }

    context(c: Ctx)
    val Int.memExt: String
        get() = "memExt $this $m ${c.n}"

    context(c: Ctx)
    open fun describe(): String = "M $m ${c.n}"
}

abstract class Shape<T> {
    context(t: T)
    abstract fun area(): Int
}

// A safe call of a function that returns Unit is null if its receiver
// is, as a Unit?: ?: runs its right side then, as with a shared
// preference's prefs?.edit { ... } ?: run { field = value }.
inline fun StringBuilder.edit(commit: Boolean = false, action: StringBuilder.() -> Unit) {
    action()
    if (commit) append("!")
}

fun StringBuilder.plain(x: Int) {
    append(x)
}

fun show(x: Any?) = println(x)

class Pref(val sb: StringBuilder?) {
    var token: String? = null
        get() = sb?.toString()?.takeIf { it.isNotEmpty() } ?: field
        set(value) {
            sb?.edit { append(value) } ?: run {
                println("fallback")
                field = value
            }
        }
}

fun main() {
    val p = Pref(null)
    p.token = "t"
    println(p.token)
    val q = Pref(StringBuilder())
    q.token = "u"
    println(q.token)
    val s: StringBuilder? = null
    s?.plain(1) ?: println("a")
    s?.edit { } ?: println("b")
    StringBuilder().plain(2) ?: println("never")
    val l: Any? = s?.plain(1)
    println(l)
    show(s?.plain(1))
    show(StringBuilder()?.plain(1))
}

// A member that doesn't take a call's arguments gives way to an
// extension with its name, as in Kotlin.

class Box(val v: Int) {
    fun show(prefix: String): String = prefix + v

    fun add(x: Int): String = "member " + (v + x)
}

fun Box.show(): String = "box " + v

fun Box.add(s: String): String = "extension " + s

class Tags {
    val m = java.util.TreeMap<String, Any?>()

    fun tag(key: String, tag: Any?): Tags {
        m.put(key, tag)
        return this
    }
}

// The parameter tag can't be invoked, so tag(...) calls the member.
fun Tags.tag(tag: Any?): Tags = tag("default", tag)

fun main() {
    val b = Box(1)
    println(b.show("v="))
    println(b.show())
    println(b.add(2))
    println(b.add("two"))
    val sb = StringBuilder()
    sb.append(1).append("a")
    println(sb.toString())
    println(Tags().tag(1).tag("k", 2).m)
}

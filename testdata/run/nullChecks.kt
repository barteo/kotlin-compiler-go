// Null checks: of the parameters of functions that Java code calls with
// null (here through reflection), and of values from Java used as
// non-null types. The messages are kotlinc's.

class Box(var name: String) {
    fun put(s: String, n: Int, t: String?) = s
    private fun hidden(s: String) = "hidden " + s
    constructor(a: String, b: Int) : this(a)
}

data class Item(val a: String, val b: Int)

fun top(s: String) = s
fun String.ext(x: String) = this + x
fun show(s: Any) = "" + s

fun attempt(label: String, f: () -> Unit) {
    try {
        f()
        println(label + ": ok")
    } catch (e: NullPointerException) {
        println(label + ": " + e.message)
    } catch (e: java.lang.reflect.InvocationTargetException) {
        println(label + ": " + e.cause?.message)
    }
}

fun main() {
    val str = String::class.java
    val int = Integer.TYPE
    val facade = Class.forName("NullChecksKt")
    attempt("top") { facade.getMethod("top", str).invoke(null, null) }
    attempt("ext") { facade.getMethod("ext", str, str).invoke(null, null, "x") }
    attempt("ext arg") { facade.getMethod("ext", str, str).invoke(null, "x", null) }
    attempt("member") { Box::class.java.getMethod("put", str, int, str).invoke(Box("b"), null, 1, null) }
    attempt("nullable") { Box::class.java.getMethod("put", str, int, str).invoke(Box("b"), "s", 1, null) }
    attempt("private") {
        val m = Box::class.java.getDeclaredMethod("hidden", str)
        m.setAccessible(true)
        println(m.invoke(Box("b"), null))
    }
    attempt("setter") { Box::class.java.getMethod("setName", str).invoke(Box("b"), null) }
    attempt("constructor") { Box::class.java.getConstructor(str).newInstance(null) }
    attempt("secondary") { Box::class.java.getConstructor(str, int).newInstance(null, 1) }
    attempt("copy") { Item::class.java.getMethod("copy", str, int).invoke(Item("a", 1), null, 2) }

    attempt("platform") {
        val s: String = System.getProperty("kc.none")
        println(s)
    }
    attempt("argument") { println(top(System.getProperty("kc.none"))) }
    attempt("any") { println(show(System.getProperty("kc.none"))) }
    attempt("branch") {
        val t: String = if (System.currentTimeMillis() > 0) System.getProperty("kc.none") else "b"
        println(t)
    }
    attempt("variable") {
        val p = System.getProperty("kc.none")
        val q: String = p
        println(q)
    }
    attempt("nullable platform") {
        val n: String? = System.getProperty("kc.none")
        println(n)
    }
    attempt("field") {
        val sep: String = java.io.File.separator
        println(sep.length)
    }
    attempt("bang") {
        val n: String? = System.getProperty("kc.none")
        println(n!!)
    }
}

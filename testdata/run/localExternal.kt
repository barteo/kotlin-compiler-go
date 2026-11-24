// Local external functions: native methods of the enclosing class, named
// as a local function's are (f$g), which throw UnsatisfiedLinkError when
// called, as no library registers them.

fun f(x: Int): Int {
    external fun g(a: Int): Int
    return try {
        g(x)
    } catch (e: UnsatisfiedLinkError) {
        println("g: " + e.message)
        -1
    }
}

fun defaults(a: Int) {
    external fun d(b: Int = a, s: String = "s"): String
    d()
}

class C {
    fun k() {
        external fun g(): String
        g()
    }

    fun twice() {
        if (true) {
            external fun h()
            h()
        } else {
            external fun h(x: Long)
            h(1L)
        }
    }
}

object O {
    fun o() {
        val l = {
            external fun inLambda(): Any?
            inLambda()
        }
        l()
    }
}

fun attempt(name: String, f: () -> Unit) {
    try {
        f()
        println(name + ": returned")
    } catch (e: UnsatisfiedLinkError) {
        println(name + ": " + e.message)
    }
}

// natives prints the native methods of class name, and whether they are
// static, private or final.
fun natives(name: String) {
    val ms = Class.forName(name).declaredMethods
    val names = arrayOfNulls<String>(ms.size)
    var n = 0
    for (m in ms) {
        if (java.lang.reflect.Modifier.isNative(m.modifiers)) {
            names[n++] = java.lang.reflect.Modifier.toString(m.modifiers) + " " + m.name + m.parameterTypes.size
        }
    }
    java.util.Arrays.sort(names, 0, n)
    for (i in 0 until n) {
        println("  " + names[i])
    }
}

fun main() {
    println(f(3))
    attempt("defaults") { defaults(4) }
    attempt("k") { C().k() }
    attempt("twice") { C().twice() }
    attempt("o") { O.o() }
    for (name in arrayOf("LocalExternalKt", "C", "O")) {
        println(name + ":")
        natives(name)
    }
}

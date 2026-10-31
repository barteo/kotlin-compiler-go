// External functions and accessors, native methods without code, which
// throw UnsatisfiedLinkError when called, as no library registers them.

external fun top(x: Int): String

class C {
    external fun m(a: Long): Int

    var both: Int
        external get
        external set

    val getter: Int
        external get

    var setter: Int = 1
        external set

    companion object {
        @JvmStatic external fun s(x: Int)
        external fun t()
    }
}

object O {
    external fun o()
    @JvmStatic external fun so(): String?
}

open class A {
    open external fun f()
}

class B : A() {
    override fun f() {
        print("B.f, ")
        super.f()
    }
}

interface I {
    fun i(): Int
}

class D : I {
    override external fun i(): Int
}

fun attempt(name: String, f: () -> Unit) {
    try {
        f()
        println(name + ": returned")
    } catch (e: UnsatisfiedLinkError) {
        println(name + ": " + e.message)
    }
}

// natives prints the methods of class name, and whether they are static or
// native, or only the native ones.
fun natives(name: String, onlyNative: Boolean) {
    val ms = Class.forName(name).declaredMethods
    val names = arrayOfNulls<String>(ms.size)
    var n = 0
    for (m in ms) {
        val native = java.lang.reflect.Modifier.isNative(m.modifiers)
        if (native || !onlyNative) {
            val static = if (java.lang.reflect.Modifier.isStatic(m.modifiers)) "static " else ""
            names[n++] = static + (if (native) "native " else "") + m.name
        }
    }
    java.util.Arrays.sort(names, 0, n)
    for (i in 0 until n) {
        println("  " + names[i])
    }
}

fun main() {
    attempt("top") { top(1) }
    val c = C()
    attempt("m") { c.m(2L) }
    attempt("get both") { c.both }
    attempt("set both") { c.both = 3 }
    attempt("getter") { c.getter }
    attempt("get setter") { println(c.setter) }
    attempt("set setter") { c.setter = 4 }
    attempt("s") { C.s(5) }
    attempt("t") { C.t() }
    attempt("o") { O.o() }
    attempt("so") { O.so() }
    attempt("B.f") { B().f() }
    attempt("i") { (D() as I).i() }
    for (name in arrayOf("C", "C\$Companion", "O", "A", "D", "ExternalKt")) {
        println(name + ":")
        natives(name, name == "ExternalKt")
    }
}

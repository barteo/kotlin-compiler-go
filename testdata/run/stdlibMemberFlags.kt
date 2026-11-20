// What Java callers and other modules link against: an abstract method
// with a vararg parameter is a variable arity method, as one with a body
// is; a companion's protected @JvmStatic members and consts are protected
// in the outer class; and the copy of a data class with
// @ConsistentCopyVisibility has its constructor's visibility, and the
// module's name in its own if that is internal.
import java.lang.reflect.Modifier

interface Sender {
    fun send(vararg parts: String)
    fun sendInts(tag: String, vararg xs: Int): Int
    fun withBody(vararg xs: Long) {}
}

abstract class Base {
    abstract fun many(vararg xs: Any?)
    protected abstract fun hidden(vararg xs: String)
    open fun open(vararg xs: Long) {}
}

open class Outer {
    companion object {
        @JvmStatic protected fun prot() = 1
        @JvmStatic protected val protVal = 2
        @JvmStatic protected var protVar = 3
        @JvmStatic fun pub() = 4
        protected const val PROT_CONST = 5
        const val CONST = 6
    }
}

class Sub : Outer() {
    fun sum() = prot() + protVal + protVar + PROT_CONST
}

@ConsistentCopyVisibility
data class Internal internal constructor(val a: Int, val b: String) {
    fun bump() = copy(a = a + 1)
}

@ConsistentCopyVisibility
data class Protected protected constructor(val a: Int) {
    fun bump() = copy(a = a + 1)

    companion object {
        fun of(a: Int) = Protected(a)
    }
}

@ConsistentCopyVisibility
data class Published @PublishedApi internal constructor(val a: Int)

@ConsistentCopyVisibility
data class Public(val a: Int)

data class Plain internal constructor(val a: Int)

fun main() {
    for (c in listOf(Sender::class.java, Base::class.java)) {
        for (m in c.declaredMethods.filter { !it.isSynthetic }.sortedBy { it.name }) {
            println("${c.simpleName}.${m.name}: varargs ${m.isVarArgs}")
        }
    }
    for (m in Outer::class.java.declaredMethods.filter { !it.isSynthetic }.sortedBy { it.name }) {
        println("Outer.${m.name}: ${Modifier.toString(m.modifiers)}")
    }
    for (f in Outer::class.java.declaredFields.filter { it.name.endsWith("CONST") }.sortedBy { it.name }) {
        println("Outer.${f.name}: ${Modifier.toString(f.modifiers)}")
    }
    println(Sub().sum())
    for (c in listOf(Internal::class.java, Protected::class.java, Published::class.java, Public::class.java, Plain::class.java)) {
        val copies = c.declaredMethods.filter { it.name.startsWith("copy") }.sortedBy { it.name }
        println(c.simpleName + ": " + copies.joinToString { m ->
            Modifier.toString(m.modifiers and Modifier.methodModifiers()) + " " + m.name.replace(Regex("""\$(?!default).*?(?=\$|$)"""), "\\\$module")
        })
    }
    println(Internal(1, "x").copy(b = "y").bump())
    println(Protected.of(1).bump())
    println(Published(1).copy(2))
}

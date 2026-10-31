// @JvmStatic and @JvmField in objects and companion objects.

import java.lang.reflect.Modifier

open class Base(val n: Int)

object Registry : Base(3) {
    @JvmField val names = ArrayList<String>()
    @JvmField var count = 0
    @JvmField var total = 10L
    private val secret = 7
    var label = "reg"
    lateinit var late: String
    val doubled = { secret * 2 }

    init {
        names.add("first")
        count = names.size
        println("init " + this.n + " " + count)
    }

    @JvmStatic fun register(name: String, times: Int = 1): Int {
        repeat(times) { names.add(name) }
        count += times
        return count + secret
    }

    @JvmStatic private fun hidden() = secret + n

    @JvmStatic fun callHidden() = hidden()

    @JvmStatic val size: Int get() = names.size

    @JvmStatic var tag = "t"

    @JvmStatic fun lateSet() = this::late.isInitialized

    fun plain() = register("plain") + size

    class Nested {
        fun peek() = secret
    }
}

class Holder(val v: Int) {
    companion object {
        @JvmField val EMPTY = Holder(0)
        @JvmField var made = 0
        @JvmField var big = 1L
        @JvmStatic val shared = 5
        @JvmStatic var mode = "a"
        private val hidden = 9

        @JvmStatic fun make(v: Int): Holder {
            made++
            return Holder(v + hidden)
        }
    }
}

interface Defaults {
    companion object {
        @JvmField val ITEMS = listOf(1, 2)
    }
}

fun isStatic(c: Class<*>, name: String, vararg params: Class<*>) =
    Modifier.isStatic(c.getDeclaredMethod(name, *params).modifiers)

fun staticField(c: Class<*>, name: String) = Modifier.isStatic(c.getDeclaredField(name).modifiers)

fun main() {
    println(Registry.names)
    println(Registry.register("a"))
    println(Registry.register("b", 2))
    println(Registry.count)
    Registry.count += 5
    Registry.total += 3
    Registry.total *= 2
    println(Registry.count.toString() + " " + Registry.total)
    println(Registry.callHidden())
    println(Registry.size)
    Registry.tag += "x"
    println(Registry.tag)
    println(Registry.doubled())
    println(Registry.lateSet())
    Registry.late = "set"
    println("${Registry.lateSet()} ${Registry.late}")
    println(Registry.plain())
    println(Registry.label + " " + Registry.Nested().peek())
    val ref = Registry::register
    println(ref("r", 1))

    println(Holder.EMPTY.v)
    println(Holder.make(1).v)
    Holder.made += 10
    Holder.big += 1
    println(Holder.made.toString() + " " + Holder.big)
    println(Holder.shared)
    Holder.mode += "b"
    println(Holder.mode)
    println(Defaults.ITEMS)

    val r = Registry::class.java
    println(isStatic(r, "register", String::class.java, Int::class.javaPrimitiveType!!))
    println(isStatic(r, "getSize") && isStatic(r, "getTag") && isStatic(r, "setTag", String::class.java))
    println(isStatic(r, "plain"))
    println(staticField(r, "names") && staticField(r, "secret"))
    println(r.getField("count").get(null))
    val h = Holder::class.java
    println(isStatic(h, "make", Int::class.javaPrimitiveType!!) && isStatic(h, "getShared") && isStatic(h, "setMode", String::class.java))
    println((h.getField("EMPTY").get(null) as Holder).v)
    println(h.getMethod("getMode").invoke(null))
    println(Defaults::class.java.getField("ITEMS").get(null))
}

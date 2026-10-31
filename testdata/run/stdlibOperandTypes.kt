// Values whose JVM types the verifier checks: a lambda's field, wide
// receivers, platform primitives, outer instances, boxed results of
// as?, $default results, value classes' boxes.

import kotlin.coroutines.*

fun <T> eval(fn: () -> T) = fn()

class Field {
    val v: String = "O"
        get() = eval { field } + "K"
}

class Wide {
    val Long.next: Long
        get() = this + 1

    fun next(s: Long?): Long? = s?.next
}

var total = 0L
var Long.acc: Long
    get() = total
    set(v) {
        total += this + v
    }

fun accumulate(): Long {
    2L.acc -= 1
    return total
}

class Outer {
    val ok = "outer"

    inner class A {
        inner class B {
            inner class C {
                val y = ok
            }
        }
    }

    inner class Inner<I : CharSequence>(val c: I) {
        inline fun len() = ok.length + c.length
    }
}

interface Callback {
    fun invoke(): String
}

open class Base(val callback: Callback)

class Host {
    val ok = "captured"

    inner class I1 {
        inner class I2 : Base(object : Callback {
            override fun invoke() = ok
        })
    }
}

class BooleanWrap(private val value: Boolean) : Comparable<Boolean> by value

open class Player(val name: String)
class SlashPlayer(name: String) : Player(name)

abstract class Game<T : Player> {
    abstract fun player(name: String, create: Boolean = true): T?
}

class SimpleGame : Game<SlashPlayer>() {
    override fun player(name: String, create: Boolean): SlashPlayer? = if (create) SlashPlayer(name) else null
}

class Context {
    infix fun Any.was(n: Int) = "was$n"
}

suspend fun <R> Context.call(f: suspend Context.() -> R): R = f()

@JvmInline
value class Z(val x: Int) {
    fun hash(): Int = super.hashCode()

    @Suppress("INNER_CLASS_INSIDE_VALUE_CLASS")
    inner class In(val z: Z) {
        val sum = x + z.x
    }
}

@JvmInline
value class Wrap<T : Z>(val z: T)

interface Named {
    fun name(): String = "named"
    fun Long.tag() = "tag$this:" + name()
}

@JvmInline
value class Id(val s: String) : Named {
    override fun name() = s + super<Named>.name()
}

fun Id.tagged() = 1L.tag()

inline fun <reified T> Any?.castTo(): T = this as T

fun main() {
    println(Field().v)
    println(Wide().next(10L))
    println(accumulate())

    val l = ArrayList<Int>()
    l.add(1)
    var x = l[0]
    x += 1
    x++
    ++x
    println(x)

    println(Outer().A().B().C().y)
    println(Outer().Inner("ab").len())
    println(Host().I1().I2().callback.invoke())

    println(1 as? Int)
    println(true as? Boolean)
    println(BooleanWrap(false) < true)
    println(SimpleGame().player("p")!!.name)

    var r = ""
    suspend { r = Context().call { Any() was 1 } }.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
    println(r)

    println(Z(1).In(Z(2)).sum)
    println(Wrap(Z(3)).z.x)
    println(Id("x").name())
    println(Id("y").tagged())
    println(Z(4).castTo<Z>().x)
    Z(5).hash() // an identity hash code
}

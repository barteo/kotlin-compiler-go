// A function whose expression body is a safe call of a Unit function
// returns a Unit?: null if the receiver is, and kotlin.Unit in its
// descriptor, which callers compiled by kotlinc or javac link against.
class Sink {
    fun put(x: Int) {}
}

interface Starter {
    fun start(): Unit?
}

open class Base(val sink: Sink?) {
    open fun open() = sink?.put(1)
}

class Device(sink: Sink?, val thread: Thread?) : Base(sink), Starter {
    fun plain() = sink?.put(1)
    fun java() = thread?.interrupt()
    fun paren() = (sink?.put(1))
    fun Int.ext() = sink?.put(this)
    fun <T> generic(x: T) = sink?.put(1)
    inline fun inlined() = sink?.put(1)
    suspend fun suspended() = sink?.put(1)
    fun direct() = sink!!.put(1)
    override fun start() = sink?.put(1)
    override fun open() = sink?.put(2)

    fun all(): String {
        fun local() = sink?.put(1)
        return "${plain()} ${java()} ${paren()} ${1.ext()} ${generic("a")} ${inlined()} ${start()} ${open()} ${local()}" +
            " ${plain() == null} ${plain() ?: "none"}"
    }
}

fun top(sink: Sink?) = sink?.put(1)

fun main() {
    println(Device(null, null).all())
    println(Device(Sink(), Thread.currentThread()).all())
    Thread.interrupted()
    println("${top(null)} ${top(Sink())}")
    val s: Starter = Device(null, null)
    val b: Base = Device(Sink(), null)
    println("${s.start()} ${b.open()}")
    for (m in Device::class.java.declaredMethods.filter { !it.isSynthetic && '$' !in it.name }.sortedBy { it.name }) {
        println("${m.name}: ${m.returnType.name}")
    }
    println("top: " + Class.forName("StdlibSafeUnitReturnsKt").getMethod("top", Sink::class.java).returnType.name)
}

// JVM-mapped built-in members: Number's conversions, members that a Java
// superclass implements: ArrayList's size, Enum's name and ordinal.

import java.util.concurrent.atomic.AtomicInteger

class Num(val v: Int) : Number() {
    override fun toByte(): Byte = v.toByte()
    override fun toShort(): Short = v.toShort()
    override fun toInt(): Int = v
    override fun toLong(): Long = v.toLong()
    override fun toFloat(): Float = v.toFloat()
    override fun toDouble(): Double = v.toDouble()
}

class Counter : AtomicInteger(66) {
    override fun toByte(): Byte = get().toByte()
    override fun toShort(): Short = get().toShort()
}

interface Sized { val size: Int }
class Names : java.util.ArrayList<String>(), Sized

interface Ordinaled { val ordinal: Int; val name: String }
enum class Color : Ordinaled { RED, GREEN }

fun main() {
    val n: Number = Num(65)
    @Suppress("DEPRECATION_ERROR")
    println(n.toChar())
    println(Num(66).toInt().toChar())
    println(n.toInt() + n.toLong() + n.toByte() + n.toShort())
    println(n.toFloat() + n.toDouble())
    println(Counter().toInt().toChar())
    println(Counter().toInt())

    val names = Names()
    names.add("a")
    val s: Sized = names
    println(s.size)

    val o: Ordinaled = Color.GREEN
    println(o.ordinal.toString() + " " + o.name)
}

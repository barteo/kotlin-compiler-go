package editinglib

class EditingPacket(val tag: Int = 0)
class Printer {
    fun printValue(value: String, count: Int = 1): String = value
    fun printValue(value: Int, vararg rest: Int): Int = value
}
fun packetValue(): Int = 42

operator fun EditingPacket.get(index: Int): Int = tag + index
fun EditingPacket.doubled(): Int = tag * 2
fun choose(n: Int): Int = n + 1

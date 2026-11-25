package editinglib

class EditingPacket(val tag: Int = 0)
class Printer {
    fun printValue(value: String, count: Int = 1): String = value
    fun printValue(value: Int, vararg rest: Int): Int = value
}
fun packetValue(): Int = 42

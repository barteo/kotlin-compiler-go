fun <T : Byte> byte(t: T): T = t
fun <T : Long> long(t: T): T = t

class Box<T : Short>(val s: T)

fun main() {
    println(byte(1))
    println(byte(300)) // ERROR not within its bounds
    println(long(1))
    println(Box(70000)) // ERROR not within its bounds
    val i = 1
    println(long(i)) // ERROR not within its bounds
}

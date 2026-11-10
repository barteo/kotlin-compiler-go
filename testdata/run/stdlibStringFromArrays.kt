// String(...) with an argument that is a call: String(bytes) and the
// other overloads of the standard library, not String(chars), when the
// call returns a ByteArray, from Kotlin or as a Java byte[].

fun bytesOf(s: String): ByteArray = s.toByteArray()

fun charsOf(s: String): CharArray = s.toCharArray()

fun main() {
    val s = "frame"
    println(String(s.toByteArray()))
    println(String(bytesOf("box")))
    println(String(java.util.Base64.getDecoder().decode("a2M=")))
    println(String(s.toByteArray(), Charsets.UTF_8))
    println(String(s.toByteArray(), 1, 3))
    println(String(charsOf("chars")))
    println(String(s.toCharArray(), 2, 3))
    val bytes = s.toByteArray()
    println(String(bytes).length)
}

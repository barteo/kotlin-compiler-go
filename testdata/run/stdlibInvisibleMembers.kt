@file:Suppress("INVISIBLE_REFERENCE", "INVISIBLE_MEMBER")

// Under the suppression, a library's internal constructor can be called.
fun main() {
    println(Result<String>("OK").value as String)
}

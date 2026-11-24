import java.util.Date

// The common supertype of a built-in and a Java class that are both
// Comparable is Comparable<String & Date>, whichever comes first.

fun size(m: Map<String, Any>): Int = m.size

fun main() {
    val a = listOf("x", Date(0))
    val b = listOf(Date(0), "x")
    val c = listOf(1, Date(0))
    println(a[0] is String)
    println(b[1] is String)
    println(c[0] is Int)
    println(size(mapOf("type" to "date", "default" to Date(0))))
}

// A Java class's constructor called through a library's typealias keeps
// K2's String? <: T of a nullable argument passed for its flexible
// parameter, where the class's own constructor gives String? <: T?:
// HashSet(setOf(null)) is a HashSet<Nothing?>, java.util.HashSet(setOf(null))
// a HashSet<String> where a Set<String> is expected.

fun takeSet(s: Set<String>) {}
fun takeList(l: List<String>) {}
fun takeMap(m: Map<String, String>) {}

fun accepted(s: String?) {
    takeSet(java.util.HashSet(setOf(null)))
    takeList(java.util.ArrayList(listOf(s)))
    takeMap(java.util.HashMap(mapOf(s to "")))
    val h: java.util.HashSet<String> = java.util.HashSet(setOf(s))
}

fun rejected(s: String?) {
    takeSet(HashSet(setOf(null))) // ERROR type mismatch
    takeList(ArrayList(listOf(s))) // ERROR type mismatch
    takeMap(HashMap(mapOf(s to ""))) // ERROR type mismatch
    takeSet(LinkedHashSet(listOf(s))) // ERROR type mismatch
    takeSet(kotlin.collections.HashSet(setOf(null))) // ERROR type mismatch
    val h: HashSet<String> = HashSet(setOf(s)) // ERROR type mismatch
}

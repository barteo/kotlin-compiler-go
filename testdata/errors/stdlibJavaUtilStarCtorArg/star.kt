// java.util.* and an explicit import name the Java classes, ahead of
// kotlin.collections' typealiases of them: their constructors take a
// nullable argument for an equality constraint T = String.

import java.util.*
import java.util.ArrayList

fun takeSet(s: Set<String>) {}
fun takeList(l: List<String>) {}

fun accepted(s: String?) {
    takeSet(HashSet(setOf(null)))
    takeList(ArrayList(listOf(s)))
}

fun rejected(s: String?) {
    val l: List<String> = Collections.singletonList(s) // ERROR type mismatch
}

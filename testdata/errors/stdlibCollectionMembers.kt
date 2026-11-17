// The members of Java's collections that Kotlin's collections declare
// take Kotlin's parameter types, as K2's mapped scope has them: an
// ArrayList<String>'s contains takes a String!, so contains(1) falls back
// to the stdlib's contains, whose @OnlyInputTypes T can't be Any.

fun members(al: java.util.ArrayList<String>, hm: java.util.HashMap<String, Int>) {
    al.contains("")
    al.indexOf("")
    hm.containsKey("")
    hm.get("")
    al.contains(1) // ERROR must be mentioned in input types
    1 in al // ERROR must be mentioned in input types
    al.lastIndexOf(1) // ERROR must be mentioned in input types
    val ref: (String) -> Boolean = al::contains
}

class Names : java.util.ArrayList<String>()

fun subclass(n: Names) {
    n.contains("")
    // remove(int index) is the stdlib's deprecated remove(index), level
    // ERROR: Kotlin calls it removeAt.
    n.remove(1) // ERROR 'remove' is deprecated with level ERROR
    n.containsAll(listOf(1)) // ERROR must be mentioned in input types
}

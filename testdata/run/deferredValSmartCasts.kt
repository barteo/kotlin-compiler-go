// A val declared without a value is narrowed by the branch that assigns
// it, when the other branches jump, as in the code Navigation's Safe Args
// generates.

fun lookup(key: String): Any? = if (key == "name") "kc" else null

fun fromBundle(has: Boolean, key: String): String {
    val result: String?
    if (has) {
        if (key != "") {
            result = lookup(key) as String?
        } else {
            throw UnsupportedOperationException("empty key")
        }
        if (result == null) {
            throw IllegalArgumentException("Argument \"$key\" is null")
        }
    } else {
        throw IllegalArgumentException("Required argument \"$key\" is missing")
    }
    return result
}

fun assignedThenChecked(n: Int): Int {
    val x: Any
    if (n > 0) {
        x = "positive"
    } else {
        x = n
    }
    return if (x is String) x.length else -1
}

fun whenAssigned(n: Int): String {
    val s: String?
    when (n) {
        0 -> s = "zero"
        1 -> s = "one"
        else -> return "many"
    }
    return s + "!"
}

fun main() {
    println(fromBundle(true, "name"))
    try {
        fromBundle(true, "")
    } catch (e: UnsupportedOperationException) {
        println(e.message)
    }
    try {
        fromBundle(true, "other")
    } catch (e: IllegalArgumentException) {
        println(e.message)
    }
    try {
        fromBundle(false, "name")
    } catch (e: IllegalArgumentException) {
        println(e.message)
    }
    println(assignedThenChecked(3))
    println(assignedThenChecked(-3))
    println(whenAssigned(0) + whenAssigned(1) + whenAssigned(2))
}

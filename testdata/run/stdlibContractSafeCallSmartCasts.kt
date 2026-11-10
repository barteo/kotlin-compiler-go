// A contract that its argument isn't null, as isNullOrEmpty's when it
// returns false, smart casts the receivers of the argument's safe calls
// too, and those of the safe calls a local val holds.

class Box(val name: String?) {
    fun get(key: String): String? = if (key.isEmpty()) null else "$key:$name"
}

fun direct(box: Box?): Int {
    if (box?.get("a").isNullOrEmpty()) return 0
    return box.get("b")?.length ?: -1
}

fun throughLocal(box: Box?): String {
    val token = box?.get("a")?.trim()?.lowercase()
    if (token.isNullOrEmpty()) return "none"
    return token + " " + box.name
}

fun blank(box: Box?): String {
    val text = box?.name
    if (text.isNullOrBlank()) return "blank"
    return box.get(text) ?: "?"
}

fun required(box: Box?): String {
    requireNotNull(box?.name)
    return box.get("r") ?: "?"
}

fun checked(box: Box?): String {
    val name = box?.name
    checkNotNull(name)
    return box.get(name) ?: "?"
}

fun main() {
    println(direct(Box("x")))
    println(direct(null))
    println(throughLocal(Box("Y")))
    println(throughLocal(null))
    println(blank(Box("z")))
    println(blank(Box(" ")))
    println(blank(null))
    println(required(Box("q")))
    println(checked(Box("w")))
    println(runCatching { required(null) }.isFailure)
}

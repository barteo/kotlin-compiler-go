operator fun StringBuilder.plusAssign(s: String) {
    append(s)
}

fun narrowed(x: Any?): Any? {
    var sb: Any? = x
    if (sb is StringBuilder) sb += "!"
    return sb
}

fun blocks(tags: List<String>): Map<String, List<String>> {
    val blocks = linkedMapOf<String, MutableList<String>>()
    var current: MutableList<String>? = null
    for (tag in tags) {
        when {
            tag.startsWith("<") -> current = blocks.getOrPut(tag.drop(1)) { mutableListOf() }
            tag == ">" -> current = null
            current != null -> current += tag
        }
    }
    return blocks
}

fun main() {
    println(blocks(listOf("<a", "1", "2", ">", "3", "<b", "4", "<a", "5", ">")))
    var list: MutableList<Int>? = null
    if (list == null) list = mutableListOf()
    list += 1
    list -= 2
    println(list)
    println(narrowed(StringBuilder("x")))
    println(narrowed("y"))
}

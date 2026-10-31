// Raw strings, string literals inside templates, non-ASCII names,
// primary constructors with a visibility modifier, and a Double without
// digits before the point, .5.

class Größe public constructor(val n: Int) {
    fun show(): String = """n=$n
  "quoted" \n ${if (n > 1) "big}" else "{small"} $${n}"""
}

class Holder
internal constructor(val label: String)

fun main() {
    val s = "abc"
    println(!s.isEmpty())
    println("""a${"""b"""}c""""")
    println("""""")
    println("${"x" + "y"} ${"}"} ${"{"}")
    println(Größe(2).show())
    println(Größe(1).show())
    val π = 3
    println(π + 1)
    println(Holder("h").label)
    val multi = """
        |first
        |second
    """
    println(multi.length)
    println(.5 + .25f)
    for (i in 0..2) print(i)
    println()
}

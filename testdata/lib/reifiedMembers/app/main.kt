import rm.Count
import rm.Shelf
import rm.Tag
import rm.kindTop
import rm.slotTop
import rm.stored

fun firstUpper(t: Tag): Char {
    t.each { if (it.isUpperCase()) return it }
    return '-'
}

fun main() {
    val t = Tag("box")
    println("${t.matches<String>("s")} ${t.matches<Int>("s")} ${Tag("").matches<String>("s")}")
    println(t.label<List<*>>())
    println(t.filter<Int>(listOf(1, "a", 2, null)))
    println("${firstUpper(Tag("abC"))} ${firstUpper(t)}")
    val nt: Tag? = t
    println(nt?.label<String>())
    println(Count(5).plusIf<Int>(1).plusIf<String>(1).n)
    val s = Shelf(1)
    with(s) {
        println("x".kind + " " + 2.kind)
        1.slots = 5
        "s".slots += 1
        println(size)
        2.slots++
        println(size)
    }
    println("x".kindTop + " " + 1.kindTop)
    1.slotTop = 3
    "s".slotTop += 1
    2.slotTop++
    println(stored)
}

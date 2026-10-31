class Node(val value: Int, val next: Node?)

class Person(val name: String, val nickname: String?)

fun length(s: String?): Int = s?.length ?: -1

fun describe(p: Person?): String {
    if (p == null) return "nobody"
    val nick = p.nickname ?: "no nickname"
    return p.name + " aka " + nick
}

fun sumList(head: Node?): Int {
    var total = 0
    var cur = head
    while (cur != null) {
        total += cur!!.value
        cur = cur!!.next
    }
    return total
}

fun parseOrNull(s: String): Int? = if (s.isEmpty()) null else s.toInt()

fun firstNonNull(a: Int?, b: Int?): Int = a ?: b ?: 0

fun main() {
    println(length("four"))
    println(length(null))
    println(describe(null))
    println(describe(Person("Grace", null)))
    println(describe(Person("Alan", "Turing")))
    val list = Node(1, Node(2, Node(3, null)))
    println(sumList(list))
    println(sumList(null))
    println(list.next?.next?.value)
    println(list.next?.next?.next?.value)

    val maybe: Int? = parseOrNull("41")
    if (maybe != null) {
        println(maybe + 1)
    }
    val none = parseOrNull("")
    println(none)
    println(none == null)
    println(maybe == 41)
    println(firstNonNull(null, 7))
    println(firstNonNull(null, null))
    println(firstNonNull(3, 7))

    val name: String? = "kotlin"
    if (name != null && name.length > 3) {
        println(name.uppercase())
    }
    val other: String? = null
    if (other == null || other.length == 0) {
        println("empty or null")
    }
    println(name?.uppercase())
    println(other?.uppercase())
    println(maybe?.toString())
    println("value: $maybe, missing: $none")

    val d: Double? = 2.5
    println(d!! * 2)
    var counter: Int? = null
    counter = 5
    println(counter)
    val forced: String? = "x"
    println(forced!!.length)
}

// Smart casts that conditions give, as in K2: a cast in the right side of
// && holds where the && is true, false || b smart-casts as b does, and a
// chain of safe calls on a local var smart-casts the properties it reads
// until the var is assigned.
class Node(val next: Node?, val name: String)

fun andCast(b: Boolean, x: Any?): Boolean {
    if (b && x as Boolean) {
        return x.not()
    }
    if (b && x as Boolean == true) {
        return x.not()
    }
    return true
}

fun orFalse(x: Any?): Int {
    if (false || x is String) {
        return x.length
    }
    return -1
}

fun varChain(start: Node?): String {
    var n = start
    if (n?.next?.next != null) {
        return n.next.next.name
    }
    return "short"
}

fun walk(start: Node?): String {
    var n = start
    var names = ""
    while (n?.next?.next != null) {
        names += n.next.next.name
        n = n.next
    }
    return names
}

fun main() {
    println(andCast(true, true))
    println(andCast(false, null))
    println(orFalse("four"))
    println(orFalse(4))
    val list = Node(Node(Node(Node(null, "d"), "c"), "b"), "a")
    println(varChain(list))
    println(varChain(Node(null, "x")))
    println(walk(list))
}

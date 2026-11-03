// A lambda that the function it is passed to calls exactly once, as its
// contract says, and that jumps out of it: the call always jumps.

class Box {
    var value = "unset"
}

fun inApply(): String {
    Box().apply {
        value = "apply"
        return value
    }
}

fun inLet(s: String?): Int {
    s.let { return it?.length ?: -1 }
}

fun main() {
    println(inApply())
    println(inLet("four"))
    println(inLet(null))
}

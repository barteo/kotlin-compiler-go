// A callable reference or class literal can break the line after ::.

class Vm {
    fun pin(s: String) = println("pin $s")
}

fun shout(s: String) = println(s.uppercase())

fun pick(vm: Vm, ok: Boolean): ((String) -> Unit)? =
    if (ok) {
        vm::
            pin
    } else null

fun main() {
    pick(Vm(), true)?.invoke("a")
    println(pick(Vm(), false))
    val f: (String) -> Unit = ::
        shout
    f("b")
    println(Vm::
        class.simpleName)
    val g = Vm::
        pin
    g(Vm(), "c")
}

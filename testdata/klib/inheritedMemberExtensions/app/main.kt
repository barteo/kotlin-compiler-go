import lib.Base
import lib.Units

// Classes of a program that declares no member extensions inherit the
// library's: a lambda's implicit receiver of such a class dispatches
// them.
class Direct : Units

class Indirect : Base()

fun main() {
    with(Direct()) { println("a".tagged(2) + " " + 9.half) }
    with(Indirect()) { println("b".tagged(1) + " " + 5.half) }
    Direct().apply { println(listOf("c", "d").map { it.tagged(1) }) }
}

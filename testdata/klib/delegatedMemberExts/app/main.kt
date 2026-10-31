import lib.Impl
import lib.Scope

// A subinterface delegated to an instance of the library's interface:
// object : FullScope, Scope by this has one method per member extension.
interface FullScope : Scope {
    fun String.full(): String
}

fun Scope.full(): FullScope = object : FullScope, Scope by this {
    override fun String.full() = "full $this"
}

fun main() {
    with(Impl().full()) {
        println("a".weight(2f))
        println("b".full())
        println(plain())
        println("c".align())
    }
}

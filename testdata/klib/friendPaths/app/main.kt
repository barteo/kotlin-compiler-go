// friend
import lib.*

class LocalService : Service() {
    override fun name() = "local"
}

fun main() {
    val s = Secret(3)
    s.label = "changed"
    println(listOf(s.code, s.twice(), s.label, s.open()))
    println(Secret.make().code + Secret.MAX)
    println(listOf(hidden(1), hiddenVal, "hey".shout))
    counter += 2
    counter++
    println(counter)
    val c: Code = Secret(4)
    println(c.twice())
    println(Registry.names)
    println(listOf(Public(2).peek(), Public().inner))
    println(LocalService().describe())
}

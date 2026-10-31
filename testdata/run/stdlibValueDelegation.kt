interface Greeter {
    fun greet(name: String = "you"): String
    fun shout(): String = greet().uppercase()
    val polite: Boolean get() = true
}

class Plain(private val prefix: String) : Greeter {
    override fun greet(name: String) = "$prefix, $name"
}

@JvmInline
value class Loud(val g: Greeter) : Greeter by g

@JvmInline
value class Bounded<T : Greeter>(val g: T) : Greeter by g {
    fun twice() = greet() + "; " + greet("again")
}

@JvmInline
value class Names(val list: MutableList<String>) : MutableList<String> by list

@JvmInline
value class Lookup(val map: MutableMap<String, Int>) : MutableMap<String, Int> by map

class Wrapped(val list: MutableList<String>) : MutableList<String> by list

fun describe(g: Greeter) = "${g.greet()} ${g.greet("Bo")} ${g.shout()} ${g.polite}"

fun main() {
    val l = Loud(Plain("Hi"))
    println("${l.greet()} ${l.greet("Ann")} ${l.shout()} ${l.polite}")
    println(describe(l))
    val b = Bounded(Plain("Yo"))
    println(b.twice())
    println(describe(b))

    val n = Names(mutableListOf("a", "b"))
    n.addAll(0, listOf("x", "y"))
    n.add("c")
    println("${n.removeAt(1)} ${n.size} $n ${n.list}")
    val asList: MutableList<String> = n
    println("${asList.removeAt(0)} ${asList.size} ${asList.contains("b")}")
    val c: Collection<String> = n
    println("${c.size} ${c.toTypedArray().joinToString()}")

    val m = Lookup(mutableMapOf("a" to 1))
    m.putAll(mapOf("b" to 2))
    m["c"] = 3
    println("${m.size} ${m.keys} ${m.values} ${m["b"]} ${m.map}")

    val w = Wrapped(mutableListOf("p", "q", "r"))
    val wl: MutableList<String> = w
    println("${w.removeAt(0)} ${wl.removeAt(0)} ${w.size} ${w.list}")
}

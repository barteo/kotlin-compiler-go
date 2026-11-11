import l.ensure

sealed interface Ev {
    data class Go(val route: String) : Ev
    object Stop : Ev
}

fun main() {
    val e: Ev = Ev.Go("a")
    ensure(e is Ev.Go)
    println(e.route)
    val f: Any = "xyz"
    ensure(f is String)
    println(f.length)
}

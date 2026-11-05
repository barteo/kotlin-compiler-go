// In an inner class's call of its superclass's constructor, the
// superclass's companion comes before the outer class's this; in its
// body, the outer class's this comes first.
open class Outer(val fn: (() -> String)?) {
    companion object {
        val ok = "companion"
    }

    val ok = "outer"

    inner class Inner : Outer({ ok })
    inner class Body : Outer(null) {
        val x = ok
    }
}

fun main() {
    println(Outer(null).Inner().fn?.invoke())
    println(Outer(null).Body().x)
}

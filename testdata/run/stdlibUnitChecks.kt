// Members of Unit values, which are the kotlin.Unit object's, and is and
// as with Unit, which check for that object.
fun act() {}
fun main() {
    val u = act()
    println(u.toString())
    println(u.hashCode() == Unit.hashCode())
    println(Unit.toString())
    println(u.equals(Unit))
    println(act().toString())
    println(u is Unit)
    val a: Any = Unit
    println(a is Unit)
    println(a as Unit)
    println(4 as? Unit)
    println((a as? Unit).toString())
}

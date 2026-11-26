// A Java class that isn't a collection implements a collection's members
// for a subclass that is one: K2 sees the collection's
// remove(element: String) for AImpl's remove(Object), not a remove that
// takes an Int.
abstract class X : A()

fun test(x: X) {
    x.remove("")
    x.remove(1) // ERROR deprecated
    val b: Boolean = x.contains("")
}

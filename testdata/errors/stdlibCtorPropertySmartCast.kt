// In property initializers and init blocks, the name of a primary
// constructor parameter that declares a property is the property's: an
// open or mutable one takes no smart cast there, as in kotlinc.

open class Open(open val x: Any) {
    init {
        if (x is String) println(x.length) // ERROR unresolved reference: length
    }
}

class Mutable(var y: Any) {
    val z = if (y is String) y.length else 0 // ERROR unresolved reference: length
}

class Stable(val s: Any, t: Any) {
    init {
        if (s is String) println(s.length)
        if (t is String) println(t.length)
    }
}

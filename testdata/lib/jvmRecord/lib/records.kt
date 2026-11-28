// kotlinc: -jvm-target 17
package records

// Records, whose accessors are named as their properties, x(): a program
// compiled against them finds those names in their metadata.

interface Shape {
    val name: String
    val area: Double get() = 0.0
}

@JvmRecord
data class Rect(val w: Double, val h: Double) : Shape {
    override val name: String get() = "rect"
    override val area: Double get() = w * h
    val square: Boolean get() = w == h
    var scale: Int
        get() = 1
        set(value) { println("scale $value") }
    val String.tagged: String get() = "$this:$name"
    fun tag(s: String) = s.tagged
}

@JvmRecord
data class Pair2<A, B>(val first: A, val second: B) {
    companion object {
        fun <T> twice(t: T) = Pair2(t, t)
    }
}

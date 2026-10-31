package unit

// Value classes whose functions' names a hash of their parameters'
// types mangles, as Compose's Dp and MeasurePolicy.
@JvmInline
value class Dp(val value: Float) {
    operator fun div(other: Float): Dp = Dp(value / other)
    operator fun div(other: Int): Dp = Dp(value / other)
    operator fun div(other: Dp): Float = value / other.value
}

val Int.dp: Dp get() = Dp(toFloat())

@JvmInline
value class Constraints(val packed: Long) {
    val max: Int get() = packed.toInt()

    // A member with default values: a static method of the class, and its
    // copy-...$default, that take the value first.
    fun copy(min: Int = 0, max: Int = this.max): Constraints = Constraints(max.toLong() * 1000 + min)

    val min: Int get() = (packed % 1000).toInt()
}

class Scope(val name: String) {
    fun Dp.roundToPx(): Int = value.toInt()
}

fun interface MeasurePolicy {
    fun Scope.measure(items: List<String>, constraints: Constraints): String

    fun Scope.describe(): String = "policy of $name"
}

fun layout(items: List<String>, policy: MeasurePolicy): String = with(policy) {
    Scope("scope").measure(items, Constraints(7)) + " / " + Scope("s").describe()
}

// A member extension property of a value class, in an interface: vico's
// MeasuringContext's Dp.pixels, a default method getPixels-....
interface Measuring {
    val density: Float

    val Dp.pixels: Float get() = value * density
}

class Screen(override val density: Float) : Measuring

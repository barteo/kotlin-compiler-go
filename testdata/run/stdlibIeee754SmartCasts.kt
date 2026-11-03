// Float and Double comparisons of values smart cast from Comparable, Any
// and Number compare as IEEE 754 numbers (ProperIeee754Comparisons):
// -0.0 == 0.0 and NaN != NaN, unlike Double.equals and compareTo.
fun lessF(x: Comparable<Float>, y: Float) = x is Float && x < y
fun lessD(x: Comparable<Double>, y: Double) = x is Double && x < y
fun eqAny(x: Any, y: Double) = x is Double && x == y
fun eqBoth(x: Any?, y: Any?) = x is Double && y is Double && x == y
fun ltBoth(x: Any, y: Any) = x is Double && y is Double && x < y
fun <T : Comparable<T>> generic(x: T, y: T) = x < y
fun eqNum(x: Number, y: Double) = x is Double && x == y
fun eqNullable(x: Float?, y: Float) = x == y
fun eqNullables(x: Double?, y: Double?) = x == y
fun whenCond(x: Any) = when { x is Double && x == 0.0 -> "zero"; else -> "other" }
fun whenSubj(x: Double) = when (x) { 0.0 -> "zero"; else -> "other" }
fun whenSubjAny(x: Any) = when (x) { 0.0 -> "zero"; else -> "other" }
fun mixed(x: Any, y: Float) = x is Double && x > y

fun main() {
    println("${lessF(-0.0F, 0.0F)} ${lessD(-0.0, 0.0)} ${lessD(Double.NaN, 1.0)}")
    println("${eqAny(-0.0, 0.0)} ${eqAny(Double.NaN, Double.NaN)}")
    println("${eqBoth(-0.0, 0.0)} ${eqBoth(Double.NaN, Double.NaN)}")
    println("${ltBoth(-0.0, 0.0)} ${ltBoth(Double.NaN, 1.0)}")
    println("${generic(-0.0, 0.0)} ${generic(Double.NaN, 1.0)}")
    println("${eqNum(-0.0, 0.0)} ${eqNullable(-0.0F, 0.0F)} ${eqNullable(null, 0.0F)}")
    println("${eqNullables(-0.0, 0.0)} ${eqNullables(Double.NaN, Double.NaN)} ${eqNullables(null, null)}")
    println("${whenCond(-0.0)} ${whenSubj(-0.0)} ${whenSubjAny(-0.0)}")
    println(mixed(1.5, 1.0F))
}

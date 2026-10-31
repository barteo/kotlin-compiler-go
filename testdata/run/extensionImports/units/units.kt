package units

class Meters(val v: Double)

fun Int.m(): Meters = Meters(toDouble())
fun Meters.km(): Double = v / 1000.0
fun String.tag(): String = "units:" + this

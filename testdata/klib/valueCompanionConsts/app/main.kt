// A value class's companion consts are static fields of the value class,
// as kotlinc writes them, so that kotlinc can read and fold them.

import lib.LIMIT

@JvmInline
value class Margin(val percent: Int) {
    val coerced: Int get() = percent.coerceIn(0, MAX_PERCENT)

    companion object {
        const val MAX_PERCENT = 80
        const val LABEL = "margin"
        const val SCALE = 0.5
    }
}

fun main() {
    println("${Margin(95).coerced} ${Margin.MAX_PERCENT} ${Margin.LABEL} ${Margin.SCALE} $LIMIT")
}

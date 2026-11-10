private const val MAX_FRACTION = Margin.MAX_PERCENT / 200f

private const val TITLE = Margin.LABEL + "!"

fun main() {
    println("use $MAX_FRACTION $TITLE ${Margin.SCALE * 2} ${Margin(10).coerced}")
}

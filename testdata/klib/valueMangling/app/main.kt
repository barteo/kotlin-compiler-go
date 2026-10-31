import unit.Constraints
import unit.Dp
import unit.MeasurePolicy
import unit.Measuring
import unit.Screen
import unit.dp
import unit.layout

val StrokeFraction = 4.dp / 40.dp

fun fits(width: Dp, item: Dp) = if (item.value > 0) (width / item).toInt() else 0

val Policy = MeasurePolicy { items, c -> "$name ${items.size} ${c.max} ${8.dp.roundToPx()}" }

fun Measuring.half(d: Dp): Float = d.pixels / 2f

fun main() {
    val screen = Screen(2f)
    println(screen.half(3.dp))
    println(with(screen) { 4.dp.pixels })
    println(StrokeFraction)
    println(fits(100.dp, 30.dp))
    println((10.dp / 4).value + (10.dp / 2f).value)
    println(layout(listOf("a", "b")) { items, constraints -> "$name $items ${constraints.max}" })
    println(layout(emptyList(), Policy))
    val c = Constraints(5).copy(max = 9)
    println("${c.min} ${c.max}")
    val d = c.copy(max = 4, min = 2)
    println("${d.min} ${d.max} ${c.copy().max}")
}

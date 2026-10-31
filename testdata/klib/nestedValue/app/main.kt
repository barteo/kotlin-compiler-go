import paint.Color
import paint.Colors
import paint.mix

object Palette {
    val Red = Color(0xffff0000)
    val Faded = Red.copy(alpha = 0x80)
    var current: Color = Colors.Black
}

val Blue = Color(0xff0000ff)
val maybe: Color? = Blue

class Swatch(val color: Color, val name: String)

fun describe(c: Color): String = "$c a=${c.alpha}"

fun main() {
    println(describe(Palette.Red))
    println(describe(Palette.Faded))
    println(Palette.current)
    Palette.current = mix(Palette.Red, Blue)
    println(Palette.current)
    println(maybe)
    val swatches = listOf(Swatch(Blue, "blue"), Swatch(Colors.Black, "black"))
    for (s in swatches) println(s.name + " " + s.color.value)
    println(swatches.map { it.color }.toSet().size)
}

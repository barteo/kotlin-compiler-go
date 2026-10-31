import lib.Color
import lib.Dp
import lib.Options
import lib.Shape
import lib.Solid

// The constructors of a library class that take a value class are
// private: callers call the synthetic ones that take a marker.
class Rounded(corner: Dp) : Shape(corner)

fun main() {
    println(Solid(Color(42UL)))
    println(Options())
    println(Options(Dp(3f)))
    println(Options(count = 5))
    println(Options(Dp(2f), 1, "all"))
    println(Options("named", Dp(4f)))
    println(Shape(Dp(6f)))
    println(Rounded(Dp(7f)))
}

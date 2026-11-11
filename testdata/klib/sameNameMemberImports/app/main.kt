import lib.Point
import lib.Point.Companion.describe
import Labels.describe

object Labels {
    fun String.describe(): String = "label $this"
}

fun main() {
    println(Point(1).describe())
    println("a".describe())
}

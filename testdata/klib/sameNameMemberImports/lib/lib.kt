package lib
class Point(val x: Int) {
    companion object {
        fun Point.describe(): String = "point $x"
    }
}

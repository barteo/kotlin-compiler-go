package lib.geometry

data class Point(val x: Float, val y: Float)

data class Rect(val left: Float, val top: Float, val right: Float, val bottom: Float)

fun Rect(origin: Point, size: Float): Rect = Rect(origin.x, origin.y, origin.x + size, origin.y + size)

package geo.defs

typealias Point = Pair<Int, Int>
typealias Transform = (Point) -> Point
typealias Points = java.util.ArrayList<Point>

class Shape(val name: String)
typealias Figure = Shape

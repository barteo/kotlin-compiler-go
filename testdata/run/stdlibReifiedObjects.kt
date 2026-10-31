// Inline functions whose bodies create objects that use their reified
// type parameters, as Sequence.filterIsInstance's { it is R }: each call
// gets a copy of the object's class with its type argument.

open class Shape
class Circle(val r: Int) : Shape()
class Square(val side: Int) : Shape()

inline fun <reified T> count(xs: Sequence<Any>): Int = xs.filterIsInstance<T>().count()

fun main() {
  val shapes: Sequence<Any> = sequenceOf(Circle(1), Square(2), "text", Circle(3), 4)
  println(shapes.filterIsInstance<Circle>().map { it.r }.toList())
  println(shapes.filterIsInstance<Square>().map { it.side }.toList())
  println(shapes.filterIsInstance<Shape>().count())
  println(shapes.filterIsInstance<String>().toList())
  println(count<Int>(shapes))
  println(count<Shape>(shapes))
}

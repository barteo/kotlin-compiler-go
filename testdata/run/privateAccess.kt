// Private functions and accessors are private on the JVM: other classes
// of the file call them through static accessors.

private fun twice(x: Int = 21) = x * 2
private fun String.shout() = this + "!"
private val topCustom: Int get() = 5

interface Action {
  fun run(): Int
}

open class Counter(private var count: Int) {
  private fun bump(by: Int = 1): Int {
    count += by
    return count
  }
  private fun String.tag() = "<" + this + ">"
  private val doubled: Int
    get() = count * 2
  protected open fun step() = 10

  fun tagged() = "x".tag()

  fun viaLambda(): Int {
    val f = { bump(step()) }
    return f()
  }

  fun viaObject(): Action = object : Action {
    override fun run() = bump(100) + doubled
  }

  class Nested {
    fun poke(c: Counter) = c.bump(1000) + c.doubled
  }

  companion object {
    fun make(): Counter {
      val c = Counter(1)
      c.bump()
      c.bump(2)
      c.count = 20
      return c
    }

    fun read(c: Counter) = c.tagged() + twice() + "a".shout() + topCustom
  }
}

class Sub : Counter(0) {
  override fun step() = 3
  private fun bump(by: Int = 1) = -by
  fun mine() = bump()
}

class Tags private constructor(val tags: String) {
  fun copy() = Tags(tags)

  companion object {
    fun make() = Tags("made")

    operator fun invoke(tags: String): Tags = Tags(tags + "+copied")
  }
}

fun main() {
  println(Tags("a").tags + " " + Tags("b").copy().tags + " " + Tags.make().tags)
  val c = Counter.make()
  println(c.viaLambda())
  println(c.viaObject().run())
  println(Counter.Nested().poke(c))
  println(Counter.read(c))
  val s = Sub()
  println(s.viaLambda())
  println(s.mine())
  println(twice(4) + "b".shout().length + topCustom)
}

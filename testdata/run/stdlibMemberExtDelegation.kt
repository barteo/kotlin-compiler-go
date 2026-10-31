// Generic member extensions overridden and delegated, smart casts from
// safe calls compared with values, and builder inference from locals.
interface Context {
  fun <R> List<R>.distinctFrom(n: Int = 0): Set<R>
  fun <T> first(xs: List<T>): T
}

class ContextImpl : Context {
  override fun <R> List<R>.distinctFrom(n: Int): Set<R> = drop(n).toSet()
  override fun <T> first(xs: List<T>): T = xs[0]
}

class Delegating : Context by ContextImpl() {
  fun run(): String = listOf(1, 2, 2, 3).distinctFrom(1).toString() + listOf(4, 4).distinctFrom() + first(listOf("a"))
}

class Holder(val inner: String?)

fun success(r: Result<Int>?): Int {
  if (r?.isSuccess == true) return r.getOrThrow()
  return 0
}

fun innerLength(h: Holder?): Int {
  if (h?.inner?.length != null) return h.inner.length
  if (h?.inner != "x") return -1
  return h.inner.length
}

fun three(s: String?): Int = if (s?.length == 3) s.length else -1

fun next(i: Int): String? = if (i < 3) "x$i" else null

fun collected(): List<String> {
  var i = 0
  val xs = buildList {
    while (true) {
      val e = next(i++) ?: break
      add(e)
    }
  }
  return xs
}

fun firstOnly(): List<String> {
  val xs = buildList { add(next(0)!!) }
  return xs
}

fun main() {
  println(Delegating().run())
  println(success(runCatching { 4 }))
  println(success(null))
  println(innerLength(Holder("abc")))
  println(innerLength(Holder("x")))
  println(innerLength(Holder("y")))
  println(innerLength(null))
  println(three("abc") + three(null))
  println(collected())
  println(firstOnly())
}

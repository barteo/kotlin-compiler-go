// The constructs that square/wire's runtime and gRPC client needed.

import java.io.Closeable
import java.time.Duration
import kotlin.String

// A lambda that returns Any, ending with an assignment, a when whose
// branches assign, or an if without else: Unit is an Any.
inline fun forEachTag(handler: (Int) -> Any): Int {
  var n = 0
  for (tag in 1..3) {
    handler(tag)
    n++
  }
  return n
}

fun decode(): String {
  var a = 0
  var b = ""
  forEachTag { tag ->
    when (tag) {
      1 -> a = tag * 10
      2 -> b = "two"
      else -> tag
    }
  }
  forEachTag { if (it == 3) a += it }
  forEachTag { a++ }
  val unit: Any = Unit
  return "$a $b $unit"
}

// Boolean's and, or and xor.
fun addExact(x: Long, y: Long): Long {
  val result = x + y
  if ((x xor y < 0L) or ((x xor result) >= 0L)) return result
  throw ArithmeticException()
}

// The null branch of a when sees Nothing?, and a smart-cast T is still a T.
class Adapter<T>(val name: String) {
  fun size(value: T): Int = name.length
}

val NULLS = Adapter<Nothing?>("null")

fun encodedSize(value: Any?): Int = when (value) {
  null -> NULLS.size(value)
  is String -> value.length
  else -> -1
}

fun <T> copyOfStruct(value: T): T = when (value) {
  null -> value
  is Boolean -> value
  is List<*> -> {
    @Suppress("UNCHECKED_CAST")
    value.map { copyOfStruct(it) } as T
  }
  else -> value
}

fun <T> orNull(value: T): T {
  if (value == null) return value
  return value
}

// The expected type joins the branches.
class Marked : AbstractMutableList<String>(), RandomAccess {
  private val list = ArrayList<String>()
  override val size get() = list.size
  override fun get(index: Int) = list[index]
  override fun add(index: Int, element: String) = list.add(index, element)
  override fun removeAt(index: Int) = list.removeAt(index)
  override fun set(index: Int, element: String) = list.set(index, element)
}

fun copyOf(marked: Boolean): MutableList<String> = if (marked) Marked() else ArrayList()

fun widen(value: Long): Any = when {
  value < 0L -> java.math.BigInteger.ONE.add(java.math.BigInteger.valueOf(value))
  else -> value
}

// The property's type gives lazy's.
val paths: List<String> by lazy {
  if (System.nanoTime() == 0L) emptyList() else buildList(2) { add("a"); add("b") }
}

// is and as with mutable collection types.
fun addTo(x: Any?): String = when (x) {
  is MutableList<*> -> {
    @Suppress("UNCHECKED_CAST")
    (x as MutableList<Any>).add(1)
    "list $x"
  }
  is MutableMap<*, *> -> "map"
  is MutableSet<*> -> "set"
  else -> "other"
}

fun asMap(x: Any?): MutableMap<*, *>? = x as? MutableMap<*, *>

// super.p, read and assigned.
open class Message {
  protected var hashCode = 0
}

class Any1 : Message() {
  override fun hashCode(): Int {
    var result = super.hashCode
    if (result == 0) {
      result = 42
      super.hashCode = result
    }
    super.hashCode += 1
    return result
  }
}

// An extension property of an object expression.
abstract class Sizer {
  abstract fun seconds(d: Duration): Long
}

fun sizer(): Sizer = object : Sizer() {
  override fun seconds(d: Duration): Long = d.sameSignSeconds

  private val Duration.sameSignSeconds: Long
    get() = if (getSeconds() < 0L && getNano() != 0) getSeconds() + 1L else getSeconds()
}

// The package's use(close, block) doesn't take one argument: the standard
// library's use does.
internal inline fun <C, R> C.use(close: (C) -> Unit, block: (C) -> R): R {
  try {
    return block(this)
  } finally {
    close(this)
  }
}

class Source(val name: String) : Closeable {
  override fun close() = println("closed $name")
}

fun Source.read(): String {
  use {
    Source("inner").use { inner ->
      return name + "+" + inner.name
    }
  }
}

fun custom(s: Source) = s.use({ println("custom close") }) { it.name }

// Equality with Unit, and an interface's clone().
fun <R : Any> notUnit(response: R): Boolean = response != Unit

interface Call {
  fun clone(): Call
}

class RealCall(val n: Int) : Call {
  override fun clone(): Call = RealCall(n + 1)
}

// A smart cast of this, used in a lambda with a receiver.
abstract class Client {
  fun header(sb: StringBuilder): StringBuilder {
    check(this is LimitedClient) { "no limit" }
    return sb.apply {
      if (limit < Long.MAX_VALUE) append("limit=")
      append(limit)
    }
  }
}

class LimitedClient(internal val limit: Long) : Client()

// A companion's property initializers run in order with its @JvmField
// ones, which its outer class holds.
class Status private constructor(val name: String, val code: Int) {
  companion object {
    init {
      println("init 1")
    }

    @JvmField val OK: Status = Status("OK", 0)
    private val count = 1

    @JvmField val BAD: Status = Status("BAD", count)
    private val ALL = listOf(OK, BAD).also {
      for ((i, s) in it.withIndex()) check(i == s.code)
    }
    var later = ALL.size

    init {
      println("init 2 $later")
    }

    fun get(i: Int) = ALL.getOrNull(i)
  }
}

// Parameters of type Unit and Nothing?.
abstract class Adapter2<T> {
  abstract fun size(value: T): Int
}

val units = object : Adapter2<Unit>() {
  override fun size(value: Unit): Int = 7
}

val nulls = object : Adapter2<Nothing?>() {
  override fun size(value: Nothing?): Int = 8
}

fun takeUnit(u: Unit, n: Int): Int = n + 1

fun main() {
  println(decode())
  println(Status.get(1)?.name)
  Status.later++
  println(Status.later)
  val a: Adapter2<Unit> = units
  println(a.size(Unit) + units.size(Unit))
  println(nulls.size(null))
  println(takeUnit(println("unit"), 2))
  println(addExact(2, 3))
  println(encodedSize(null))
  println(encodedSize("abc"))
  println(copyOfStruct(listOf(true, null)))
  println(orNull<String?>(null))
  println(copyOf(true).javaClass.simpleName + " " + copyOf(false).javaClass.simpleName)
  println(widen(-1))
  println(widen(5))
  println(paths)
  println(addTo(mutableListOf<Any>()))
  println(addTo(java.util.HashMap<Int, Int>()))
  println(addTo(mutableSetOf(1)))
  println(addTo("x"))
  println(asMap(mutableMapOf(1 to 2)))
  println(asMap(1))
  val m = Any1()
  println(m.hashCode())
  println(m.hashCode())
  println(sizer().seconds(Duration.ofSeconds(-3, 5)))
  println(Source("outer").read())
  println(custom(Source("c")))
  println(notUnit(Unit))
  println(notUnit("x"))
  println((RealCall(1).clone() as RealCall).n)
  println(LimitedClient(7).header(StringBuilder()))
}

// kotlinc: -jvm-default=no-compatibility
// The JVM members that Java callers and code compiled by kotlinc see,
// read by reflection: names, descriptors and flags.

import java.io.IOException
import java.lang.reflect.Member
import java.lang.reflect.Method
import java.lang.reflect.Modifier

val topVal = listOf(1)
var topVar = 2
internal var topInternal = 3
private val topPrivate = listOf(4)
private var topPrivateVar = 5
const val TOP_CONST = "a" + "b" + 1
private const val TOP_PRIVATE_CONST = 7L
@JvmField val topField = listOf(8)
@get:JvmName("fetchNamed") @set:JvmName("storeNamed") var named = 9

fun f0() = Fields(0)

fun nothing(message: String): Nothing = throw IllegalStateException(message)

@Throws(IOException::class, IllegalStateException::class)
fun throwing(fail: Boolean): Int {
  if (fail) throw IOException("io")
  return 1
}

@JvmOverloads
fun overloaded(a: Int, b: String = "b", c: Long = 3L, d: Boolean = true): String = "$a $b $c $d"

@Deprecated("", level = DeprecationLevel.HIDDEN)
fun hidden(): Int = 1

@JvmSynthetic
fun syntheticFun(): Int = 2

class Fields(@JvmField var ctorField: Int) {
  @JvmField val field = "f"
  @get:JvmName("isReady") val ready = true
  internal val internalProp = 1
  internal fun internalFun(x: Int = 2) = x + internalProp

  fun sum() = ctorField + field.length + internalFun()

  companion object {
    const val COMPANION_CONST = 10
    private const val PRIVATE_CONST = "p"
    internal const val INTERNAL_CONST = 'c'

    @JvmStatic fun create(x: Int): Fields = Fields(x)

    @JvmStatic
    @JvmOverloads
    fun join(a: String, b: String = "-", c: String = "+"): String = a + b + c + PRIVATE_CONST

    fun plain() = COMPANION_CONST
  }
}

object Constants {
  const val OBJECT_CONST = 1.5
  val notConst = listOf(OBJECT_CONST)
}

data class Point(val x: Int, val y: Int)

class Overloaded @JvmOverloads constructor(val a: Int, val b: String = "b", val c: Long = 2L) {
  @JvmOverloads constructor(s: String, t: Int = 1) : this(t, s)
}

class Private private constructor(val x: Int) {
  companion object {
    fun make() = Private(1)
  }
}

sealed class Shape(val sides: Int) {
  class Square : Shape(4)
}

abstract class Base {
  internal abstract fun describe(): String
  internal open val label: String get() = "base"
}

class Derived : Base() {
  override fun describe() = "derived " + label
  override val label: String get() = "derived"
}

@JvmDefaultWithCompatibility
interface Greeter {
  val name: String
  fun greet(): String = "hello " + name
}

class English : Greeter {
  override val name = "world"
}

fun access(): Int = topPrivate.size + topPrivateVar

class UsesPrivate {
  fun read() = topPrivate.size + topPrivateVar++ + TOP_PRIVATE_CONST.toInt()
}

fun flags(m: Int, synthetic: Boolean, varargs: Boolean = false): String {
  var s = Modifier.toString(m)
  if (synthetic) s += " synthetic"
  if (varargs) s += " varargs"
  return s.trim()
}

fun show(cls: Class<*>, vararg names: String) {
  val methods = cls.declaredMethods.filter { it.name in names }
    .map { it.name + it.parameterTypes.joinToString(",", "(", ")") { p -> p.simpleName } + it.returnType.simpleName + " " + flags(it.modifiers, it.isSynthetic, it.isVarArgs) }
  val fields = cls.declaredFields.filter { it.name in names }
    .map { "field " + it.name + " " + it.type.simpleName + " " + flags(it.modifiers, it.isSynthetic) }
  for (s in (fields + methods).sorted()) println(cls.simpleName + "." + s)
}

fun ctors(cls: Class<*>) {
  val cs = cls.declaredConstructors.map {
    it.parameterTypes.joinToString(",", "(", ")") { p -> p.simpleName } + " " + flags(it.modifiers, it.isSynthetic, it.isVarArgs)
  }
  for (s in cs.sorted()) println(cls.simpleName + ".<init>" + s)
}

fun main() {
  println(topVal + topVar + topInternal + topField + TOP_CONST + access() + UsesPrivate().read() + access())
  topVar = 20
  topInternal++
  named += 1
  println("$topVar $topInternal $named ${f0().ready}")
  try {
    nothing("boom")
  } catch (e: IllegalStateException) {
    println(e.message)
  }
  try {
    throwing(true)
  } catch (e: IOException) {
    println(e.message)
  }
  println(overloaded(1) + " " + overloaded(2, "x", d = false))
  println(syntheticFun())
  val f = Fields(5)
  f.ctorField += 2
  println(f.sum().toString() + " " + f.field + " " + f.internalProp)
  println(Fields.create(3).sum().toString() + " " + Fields.join("a") + " " + Fields.plain() + " " + Fields.COMPANION_CONST + Fields.INTERNAL_CONST)
  println(Constants.OBJECT_CONST.toString() + Constants.notConst)
  val p = Point(1, 2)
  println(p.copy(y = 3).component2() + p.component1())
  println(Private.make().x)
  println(Shape.Square().sides)
  println(Derived().describe())
  println(English().greet())

  val facade = Class.forName("StdlibJvmAbiKt")
  show(facade, "topVal", "getTopVal", "topVar", "getTopVar", "setTopVar", "topInternal", "getTopInternal", "setTopInternal",
    "topPrivate", "getTopPrivate", "topPrivateVar", "TOP_CONST", "getTOP_CONST", "TOP_PRIVATE_CONST", "topField", "getTopField",
    "fetchNamed", "storeNamed", "nothing", "throwing", "overloaded", "overloaded\$default", "hidden", "syntheticFun")
  println(facade.getDeclaredMethod("throwing", Boolean::class.javaPrimitiveType).exceptionTypes.map { it.simpleName })
  println(facade.getDeclaredField("TOP_CONST").get(null))
  show(Fields::class.java, "isReady", "getReady", "ctorField", "getCtorField", "field", "getField", "internalProp", "getInternalProp\$main", "internalFun\$main",
    "internalFun\$main\$default", "COMPANION_CONST", "PRIVATE_CONST", "INTERNAL_CONST", "create", "join", "plain")
  show(Fields.Companion::class.java, "getCOMPANION_CONST", "create", "join", "plain", "join\$default")
  show(Constants::class.java, "OBJECT_CONST", "getOBJECT_CONST", "getNotConst")
  show(Point::class.java, "component1", "component2", "copy", "toString", "hashCode", "equals")
  println(Overloaded(1).b + Overloaded("s").a + Overloaded::class.java.getConstructor(Int::class.javaPrimitiveType, String::class.java).newInstance(7, "z").c)
  ctors(Overloaded::class.java)
  ctors(Private::class.java)
  ctors(Shape::class.java)
  show(Base::class.java, "describe\$main", "getLabel\$main")
  show(Derived::class.java, "describe\$main", "getLabel\$main")
  show(Greeter::class.java, "greet", "getName", "access\$greet\$jd")
  show(Class.forName("Greeter\$DefaultImpls"), "greet")
  println(Array<Unit>::class.java.simpleName)
}

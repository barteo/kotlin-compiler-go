package e

import a.Marker
import kotlin.reflect.KClass

enum class Color { RED, GREEN }

annotation class Inner(val s: String = "in", vararg val xs: Int)

@Retention(AnnotationRetention.SOURCE)
annotation class Args(
    val s: String = "dflt",
    val i: Int = 1 + 2,
    val l: Long = 7,
    val c: Char = 'c',
    val z: Boolean = false,
    val f: Float = 1.5f,
    val d: Double = 2.0,
    val u: UInt = 3u,
    val e: Color = Color.GREEN,
    val k: KClass<*> = Any::class,
    val arr: Array<String> = ["x", "y"],
    val inner: Inner = Inner("d", 1, 2),
    vararg val v: String,
)

const val C = "const"

@Args(s = C + "!", i = -4, l = 1L shl 40, c = 'q', z = true, f = 0.25f, d = 1e3, u = 0xFFFFFFFFu,
    e = Color.RED, k = String::class, arr = arrayOf("a", C), inner = Inner(xs = [5]))
class Full

@Args("pos", 9, v = ["p", "q"])
class Positional

@Args
@Inner
class Defaults

@Marker
@Args(v = *arrayOf("s1"))
fun spread() {}

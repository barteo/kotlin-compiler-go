import kotlin.reflect.KCallable
import kotlin.reflect.KClass
import kotlin.reflect.KFunction
import kotlin.reflect.KProperty

object Ext {
    @JvmStatic
    fun build(a: String?, b: String?) = "$a $b"
}

class Box(val v: Int) {
    fun twice() = v * 2
}

fun top(a: Int) = a + 1

fun names(vararg fs: KFunction<*>) = fs.joinToString { it.name }
fun names(vararg cs: KClass<*>) = cs.joinToString { it.simpleName ?: "?" }
fun names(vararg ps: KProperty<*>) = "properties " + ps.joinToString { it.name }
fun callable(c: KCallable<Int>) = c.name

fun main() {
    println(names(Ext::build, ::top))
    println(names(Box::class))
    println(names(Box::v))
    println(names(Box::twice))
    println(callable(Box::twice))
    val f: KFunction<String> = Ext::build
    println(f.name)
    val g = Ext::build
    println(g("x", null))
}

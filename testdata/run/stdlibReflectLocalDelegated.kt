// kotlin-reflect finds a local delegated property's KProperty in the
// metadata of the class or file that declares it.
import kotlin.reflect.*

class Named {
    operator fun getValue(thiz: Any?, prop: KProperty<*>): String =
        prop.name + ":" + prop.returnType
}

class Box {
    var stored = 1
    operator fun getValue(thiz: Any?, prop: KProperty<*>): Int = stored
    operator fun setValue(thiz: Any?, prop: KProperty<*>, v: Int) { stored = v }
}

class Holder {
    fun f(): String {
        val inClass by Named()
        return inClass
    }
}

class Typed {
    operator fun <T> getValue(thiz: Any?, prop: KProperty<*>): List<T> = emptyList()
}

// The type of a local of a generic function uses its type parameter.
fun <T : Comparable<T>> generic(): String {
    val inGeneric: List<T> by Typed()
    return inGeneric.toString()
}

fun main() {
    val a by Named()
    println(a)
    var b by Box()
    b = 5
    println(b)
    println(Holder().f())
    val c by lazy { 42 }
    println(c)
    println(generic<String>())
}

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

fun main() {
    val a by Named()
    println(a)
    var b by Box()
    b = 5
    println(b)
    println(Holder().f())
    val c by lazy { 42 }
    println(c)
}

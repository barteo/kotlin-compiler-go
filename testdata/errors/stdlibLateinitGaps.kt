import kotlin.reflect.KProperty

object Delegate {
    operator fun getValue(thisRef: Any?, p: KProperty<*>): String = ""
    operator fun setValue(thisRef: Any?, p: KProperty<*>, v: String) {}
}

abstract class A(lateinit var p: String) { // ERROR 'lateinit' modifier is not allowed on primary constructor parameters
    abstract lateinit var a: String // ERROR 'lateinit' modifier is not allowed on abstract properties
    lateinit var ok: String
}

interface I {
    lateinit var s: String // ERROR 'lateinit' modifier is not allowed on abstract properties
}

fun test() {
    lateinit var d: String by Delegate // ERROR 'lateinit' modifier is not allowed on properties with a custom getter or setter, or delegated ones
    lateinit var ok: String
    ok = d
    println(ok)
}

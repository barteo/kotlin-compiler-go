// Local delegated vars: reading calls the delegate's getValue, assigning
// its setValue, also with compound assignments, increments, in lambdas
// and with a provideDelegate operator.

import kotlin.reflect.KProperty

class Logged(var value: Int) {
    val log = mutableListOf<String>()
    operator fun getValue(thisRef: Any?, p: KProperty<*>): Int {
        log.add("get ${p.name}")
        return value
    }
    operator fun setValue(thisRef: Any?, p: KProperty<*>, v: Int) {
        log.add("set ${p.name}=$v")
        value = v
    }
}

class Text(var s: String) {
    operator fun getValue(thisRef: Any?, p: KProperty<*>) = s
    operator fun setValue(thisRef: Any?, p: KProperty<*>, v: String) {
        s = v
    }
}

class Named {
    operator fun provideDelegate(thisRef: Any?, p: KProperty<*>) = Text(p.name)
}

fun main() {
    val d = Logged(1)
    var x by d
    x = 5
    x += 2
    x++
    ++x
    println(x)
    println(x--)
    println(d.log)

    var s: String by Text("a")
    s += "b"
    val append = { t: String -> s += t }
    append("c")
    append("d")
    println(s)

    var name by Named()
    println(name)
    name = "changed"
    println(name)

    var m by mutableMapOf<String, Any?>("m" to 1)
    m = 2
    println(m)
}

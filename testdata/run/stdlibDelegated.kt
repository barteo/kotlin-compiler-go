// Delegated properties: by lazy, delegates with getValue and setValue
// operators (members and extensions), the standard library's delegates,
// and local delegated properties.
import kotlin.properties.Delegates
import kotlin.reflect.KProperty

class Logged<T>(private var v: T) {
    operator fun getValue(thisRef: Any?, p: KProperty<*>): T {
        println("get ${p.name}")
        return v
    }

    operator fun setValue(thisRef: Any?, p: KProperty<*>, x: T) {
        println("set ${p.name} = $x")
        v = x
    }
}

class Const(val n: Int)

operator fun Const.getValue(thisRef: Any?, p: KProperty<*>): Int = n * 10

interface Named {
    val name: String
}

class Person(first: String, last: String) : Named {
    private var computed = 0
    override val name: String by lazy {
        computed++
        "$first $last"
    }
    var age by Logged(30)
    val ten by Const(1)
    val nickname: String by lazy { name.substringBefore(' ').lowercase() }

    fun computedTimes() = computed
}

class Box<T>(v: T) {
    val first: T by lazy { v }
    val pair by lazy { Pair(first, first) }
}

object Settings {
    var level: Int by Logged(1)
    val label by lazy { "level $level" }
}

class Config(map: Map<String, Any?>) {
    val host: String by map
    val port: Int by map
}

class Mutable(val map: MutableMap<String, Any?>) {
    var title: String by map
}

val topLazy: Int by lazy {
    println("init topLazy")
    42
}
var topLogged by Logged("a")
var counter: Int by Delegates.observable(0) { p, old, new ->
    println("${p.name}: $old -> $new")
}
var late: String by Delegates.notNull<String>()

fun main() {
    val p = Person("Ada", "Lovelace")
    println(p.name)
    println(p.name)
    println(p.computedTimes())
    println(p.nickname)
    p.age = 31
    p.age += 1
    println(p.age)
    println(p.ten)
    val n: Named = p
    println(n.name)

    val b = Box("x")
    println(b.pair)

    println(Settings.label)
    Settings.level = 2
    println(Settings.level)

    val c = Config(mapOf("host" to "localhost", "port" to 8080))
    println("${c.host}:${c.port}")
    val m = Mutable(mutableMapOf("title" to "old"))
    m.title = "new"
    println(m.map)

    println(topLazy)
    println(topLazy)
    topLogged = "b"
    topLogged += "c"
    println(topLogged)
    counter = 1
    counter++
    println(counter)
    try {
        println(late)
    } catch (e: IllegalStateException) {
        println(e.message)
    }
    late = "set"
    println(late)

    val local by lazy {
        println("init local")
        "L"
    }
    val logged by Logged(7)
    println("before")
    println(local + local)
    val f = { logged * 2 }
    println(f())
    val ten: Int by Const(2)
    println(ten)

    println(Person::class.java.declaredFields.map { it.name }.sorted())
    println(Person::class.java.declaredMethods.map { it.name }.filter { !it.contains("lambda") && !it.endsWith("Computed") }.sorted()) // kc gives private properties accessors
}

// Overload resolution and inference: lazy messages of check and require, other files' private extensions, objects
// with invoke operators, properties that can't be invoked next to
// functions of their name, delegates typed by their property, smart casts
// in safe calls' arguments, member extensions with reified type
// parameters, members that give way to extensions, and type variables
// fixed in the order their bounds allow.

import kotlin.reflect.KProperty

fun checked(x: Int?, message: () -> String): Int {
    check(x != null, { "check: x is null" })
    require(x > 0, message)
    return checkNotNull(x, { "never" }) + 1
}

object Colors {
    operator fun invoke(): String = "colors"
}

val Typ = "a value"
fun Typ(n: Int): Int = n + 1

private val Pair = 3

// Delegates whose type arguments come from the property's type.
open class Frag(val args: Map<String, Any?>)

class Lz<T>(val f: () -> T) : Lazy<T> {
    override val value: T get() = f()
    override fun isInitialized() = true
}

@Suppress("UNCHECKED_CAST")
fun <T> Frag.argument(key: String): Lz<T> = Lz { args[key] as T }

@Suppress("UNCHECKED_CAST")
fun <T> plain(value: Any?): Lazy<T> = lazy { value as T }

class Checker { fun enabled(): Boolean = true }

class Screen(args: Map<String, Any?>, checker: Checker) : Frag(args) {
    val url: String by argument("url")
    val title: String? by argument("title")
    val count: Int by plain(4)
    private val enabled by lazy { checker.enabled() }
    fun show() = println("$url $title $count $enabled")
}

// Smart casts of the receiver of a safe call in its arguments.
fun shout(s: String) = println(s.uppercase())

class Banner(val description: String?) {
    fun show(extra: String?) {
        description?.let { shout(description) }
        extra?.let { shout(extra + it.length) }
        println(extra?.plus(extra.length))
    }
}

// A member extension with a reified type parameter, and one that gives
// way to an imported extension the arguments fit.
class Prefs(val m: Map<String, Any?>)

@Suppress("UNCHECKED_CAST")
inline fun <reified T> Prefs.get(key: String, default: T): T = (m[key] as? T) ?: default

class Gson

class Session(val prefs: Prefs, val tag: String) {
    inline fun <reified T> Prefs.get(key: String, gson: Gson): T? = m[key] as? T
    inline fun <reified T> Prefs.describe(key: String): String = "$tag:$key:${T::class.simpleName}:${m[key] is T}"
    val mode: Int get() = prefs.get("mode", 3)
    val name: String? get() = prefs.get("name", Gson())
    fun show() {
        println("$mode $name")
        println(prefs.describe<Int>("mode"))
        println(prefs.describe<String>("name"))
    }
}

// checkNotNull and requireNotNull pass the expected type to their value.
class Handle(val m: Map<String, Any?>) {
    @Suppress("UNCHECKED_CAST")
    operator fun <T> get(key: String): T? = m[key] as T?
}

enum class Entry { A, B }

class ViewModel(val handle: Handle) {
    val data: String = checkNotNull(handle["data"]) { "missing" }
    val entry = Entry.valueOf(checkNotNull(handle["entry"]) { "no entry" })
    val kind: Entry get() = checkNotNull(handle["kind"])
    val n: Int = requireNotNull(handle["n"])
}

// A member that the arguments' types don't fit gives way to an extension.
class Country(val iso: String)

interface ListScope {
    fun items(count: Int, key: ((Int) -> Any)? = null, content: (Int) -> Unit)
}

inline fun <T> ListScope.items(items: List<T>, noinline key: ((T) -> Any)? = null, crossinline content: (T) -> Unit) =
    items(items.size, if (key != null) { i: Int -> key(items[i]) } else null) { content(items[it]) }

object Column : ListScope {
    override fun items(count: Int, key: ((Int) -> Any)?, content: (Int) -> Unit) {
        for (i in 0 until count) {
            print("${key?.invoke(i)} ")
            content(i)
        }
    }
}

fun column(f: ListScope.() -> Unit) = Column.f()

// Type variables fixed in the order their bounds allow.
sealed class State<out T> {
    object Idle : State<Nothing>()
    class Error(val e: Throwable) : State<Nothing>()
    class Loading<T> : State<T>()
    class Success<T>(val v: T) : State<T>()
}

class Item(val n: Int)

class Box<T>(val value: T)

fun <T> Box<T>.orElse(initial: T): T = initial

// A property initialized with a call of the function or constructor of
// its name.
val Regex = Regex("a+")

class Options(var depth: Int = 0)

fun navOptions(f: Options.() -> Unit): Options = Options().apply(f)

class Activity {
    private val navOptions = navOptions { depth = 4 }
    fun show() = println("${Regex.matches("aa")} ${navOptions.depth}")
}

fun main() {
    Activity().show()
    println(checked(2) { "positive" })
    try {
        checked(null) { "positive" }
    } catch (e: IllegalStateException) {
        println(e.message)
    }
    try {
        checked(-1) { "positive" }
    } catch (e: IllegalArgumentException) {
        println(e.message)
    }
    println(previewA() + previewB())

    println(Colors())
    println(Typ(1))
    println(Pair(Pair, 2).first)

    Screen(mapOf("url" to "https://x", "title" to null), Checker()).show()
    Banner("desc").show("extra")
    Banner(null).show(null)
    Session(Prefs(mapOf("name" to "n")), "s").show()

    val vm = ViewModel(Handle(mapOf("data" to "D", "entry" to "B", "kind" to Entry.A, "n" to 4)))
    println("${vm.data} ${vm.entry} ${vm.kind} ${vm.n}")

    val countries: List<Country>? = listOf(Country("pl"), Country("uk"))
    column {
        items(countries ?: emptyList(), key = { it.iso }) { println(it.iso.uppercase()) }
        items(2) { println(it) }
    }

    val levels = Box(mapOf("a" to 1))
    println(levels.orElse(emptyMap())["a"])
    val values = sequenceOf(listOf(), listOf(Item(1)))
    println(values.toList().map { it.size })
    val states = sequenceOf(State.Idle, State.Error(Throwable("e")), State.Loading(), State.Success(Item(2)))
    println(states.count { it is State.Success })

    val actions = mutableListOf<() -> Unit>()
    actions += { println("action") }
    val pairs = mutableListOf<(Int, Int) -> Unit>()
    pairs += { a, b -> println(a + b) }
    actions.forEach { it() }
    pairs.forEach { it(1, 2) }
}

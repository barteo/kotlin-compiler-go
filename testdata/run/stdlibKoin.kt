// The constructs that koin needed.

// An inferred type argument that is a type parameter with a nullable
// bound, for a T : Any, is that parameter & Any: koin's
// new(constructor: (T1) -> R) = constructor(get()).
fun <T : Any> named(): T = "named" as T
inline fun <reified T : Any> reified(): T = (T::class.simpleName ?: "?") as T
fun <A> fromExpected(): A = named()
inline fun <reified A> fromReified(): A = reified()
fun <A> fromParam(k: (A) -> Unit) = k(named())

// A smart cast of a property of an extension's implicit receiver.
open class Factory<T>(val v: T)
class SingleFactory<T>(v: T) : Factory<T>(v)
class Definition<R>(val factory: Factory<R>)
fun start(f: SingleFactory<*>) = "start ${f.v}"
fun <T> Definition<T>.options(eager: Boolean): String =
    if (eager && factory is SingleFactory<*>) start(factory) else "lazy"

// A getter's declared type is the property's.
val <E : Enum<E>> Enum<E>.qualifier
    get(): String {
        return toString().lowercase()
    }
enum class Color { RED }

// A package's operator whose argument doesn't fit gives way to the
// standard library's: listOf(m) is a List, not a Module; and its receiver
// must be a List<Module>, not a List<T>.
class Module(val n: Int)
operator fun List<Module>.plus(m: Module): List<Module> = this + listOf(m)
fun <T> concat(xs: List<T>, ys: List<T>): List<T> = xs + ys

// A lambda ending in an if and else if without an else returns Unit.
val instances = mutableMapOf("a" to 1)
fun save(key: String, allow: Boolean, warn: Boolean): String {
    var out = ""
    instances[key]?.let {
        if (!allow) {
            out = "not allowed $it"
        } else if (warn) {
            out = "override $it"
        }
    }
    return out
}

// The expected type of a safe call's result infers a generic call's type
// arguments, also in the lambda passed to it.
fun <T> resolve(x: Any?): T? = x as T?
fun <T> viaSafeLet(x: Any?): T? = x?.let { resolve(it) }
fun viaSafeLetString(x: Any?): String? = x?.let { resolve(it) }
fun <T> viaIfElse(xs: List<Any>?): T? =
    if (xs.isNullOrEmpty()) null
    else {
        val first = xs.firstOrNull()
        first?.let { resolve(it) }
    }

// A lambda's result infers the generic call it ends with together with
// the call it is passed to, and chooses flatMap's Iterable overload.
fun <T> all(n: Int): List<T> = List(n) { "e$it" as T }
class Scope(val n: Int) {
    fun <T> getAll(): List<T> = all(n)
}
fun <T> flat(ns: List<Int>): List<T> = ns.flatMap { n -> all(n) }
fun <T> flatPlus(ns: List<Int>): List<T> = all<T>(1) + ns.flatMap { n -> all(n) }
fun flatPlusString(ns: List<Int>): List<String> = listOf("a") + ns.flatMap { all(it) }
fun <T> flatMembers(scopes: List<Scope>): List<T> = all<T>(1) + scopes.flatMap { scope -> scope.getAll() }

// A lambda's last call that takes a lambda infers itself: build's type
// argument comes from its builder lambda, not from initialize's.
class Buildee<T> {
    var v: T? = null
    fun set(x: T) { v = x }
}
fun <T> build(f: Buildee<T>.() -> Unit): Buildee<T> = Buildee<T>().apply(f)
fun <T> Buildee<T>.touch() {}
fun <T> initialize(init: () -> Buildee<T>, more: Buildee<T>.() -> Unit): Buildee<T> = init().apply(more)

// A lambda in a copy of an inline member captures that copy's receiver,
// also when another class's inline member is copied into it.
class Root(val name: String) {
    inline fun <reified T : Any> get(q: String? = null): T = "$name:${T::class.simpleName}:$q" as T
    inline fun <reified T : Any> inject(q: String? = null, mode: LazyThreadSafetyMode = LazyThreadSafetyMode.SYNCHRONIZED): Lazy<T> =
        lazy(mode) { get<T>(q) }
}
class Koin(val root: Root) {
    inline fun <reified T : Any> inject(q: String? = null, mode: LazyThreadSafetyMode = LazyThreadSafetyMode.NONE): Lazy<T> =
        root.inject(q, mode)
}

// A property's $annotations method is named after its getter, also an
// internal one's, and a property with only SOURCE annotations has one.
@RequiresOptIn
annotation class Internal

class Holder {
    @OptIn(Internal::class)
    internal val resolver: Int = 1

    @Deprecated("old")
    internal val old = 2
}

@OptIn(Internal::class)
val Int.engine: Int get() = this

// @Volatile and @Transient are their fields' modifiers, also
// kotlin.concurrent's @Volatile, an alias of kotlin.jvm's.
class Flags(@Volatile var a: Int, @Transient val b: Int) {
    @kotlin.concurrent.Volatile
    private var c = 0
    @field:Volatile
    var d = 0
}

@kotlin.concurrent.Volatile
var top = 0

fun fieldModifiers(cls: Class<*>) =
    cls.declaredFields.filter { it.name.length == 1 || it.name == "top" }.sortedBy { it.name }
        .map { it.name + ": " + java.lang.reflect.Modifier.toString(it.modifiers) }

fun annotationsMethods(cls: Class<*>) =
    cls.declaredMethods.map { it.name }.filter { it.endsWith("\$annotations") }.sorted()

fun main() {
    println(fromExpected<String>())
    println(fromReified<String>())
    fromParam<String> { println(it) }

    println(Definition(SingleFactory(1)).options(true))
    println(Definition(Factory(2)).options(true))
    println(Color.RED.qualifier)

    println((listOf(Module(1)) + Module(2)).map { it.n })
    println(concat(listOf(1), listOf(2)))

    println(save("a", false, true))
    println(save("a", true, true))
    println(save("b", true, true).isEmpty())

    println(viaSafeLet<String>("s"))
    println(viaSafeLet<String>(null))
    println(viaSafeLetString("t"))
    println(viaIfElse<String>(listOf("u")))
    println(viaIfElse<String>(emptyList()))

    println(flat<String>(listOf(1, 2)))
    println(flatPlus<String>(listOf(2)))
    println(flatPlusString(listOf(1)))
    println(flatMembers<String>(listOf(Scope(1), Scope(2))))
    println(initialize({ build { set("built") } }, { touch() }).v)

    val koin = Koin(Root("root"))
    println(koin.inject<String>("a").value)
    println(koin.inject<String>().value)
    println(koin.root.inject<String>("b").value)

    println(annotationsMethods(Holder::class.java))
    println(annotationsMethods(Class.forName("StdlibKoinKt")))

    println(fieldModifiers(Flags::class.java))
    println(fieldModifiers(Class.forName("StdlibKoinKt")))
}

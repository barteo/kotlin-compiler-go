// The constructs that clikt needed.

import arguments.convert
import arguments.long
import java.nio.file.StandardOpenOption.*
import kotlin.properties.ReadOnlyProperty
import kotlin.properties.ReadWriteProperty
import kotlin.reflect.KProperty
import options.Ctx
import options.convert
import options.int

// Function types with receivers, as typealiases, and properties of them.
typealias Conv<InT, V> = Ctx.(InT) -> V
typealias Tr<V> = Conv<String, V>
typealias All<E, A> = Ctx.(List<E>) -> A

interface Opt<AllT, EachT, ValueT> {
    val transformValue: Tr<ValueT>
    val transformAll: All<EachT, AllT>
    val n: Int
    fun <AllT, EachT, ValueT> copy(transformValue: Tr<ValueT>, transformAll: All<EachT, AllT>, n: Int = this.n): Opt<AllT, EachT, ValueT>
    fun copy(n: Int = this.n): Opt<AllT, EachT, ValueT>
}
typealias NOpt<EachT, ValueT> = Opt<EachT?, EachT, ValueT>

class Impl<AllT, EachT, ValueT>(
    override val transformValue: Tr<ValueT>,
    override val transformAll: All<EachT, AllT>,
    override val n: Int,
) : Opt<AllT, EachT, ValueT> {
    override fun <AllT, EachT, ValueT> copy(transformValue: Tr<ValueT>, transformAll: All<EachT, AllT>, n: Int): Opt<AllT, EachT, ValueT> =
        Impl(transformValue, transformAll, n)
    override fun copy(n: Int): Opt<AllT, EachT, ValueT> = Impl(transformValue, transformAll, n)
}

// transformValue(it) invokes the extension receiver's property with the
// lambda's receiver; conversion, crossinline and used in a lambda, is
// passed on to the other convert's, which is a function object.
inline fun <InT : Any, V : Any> NOpt<InT, InT>.convert(crossinline conversion: Conv<InT, V>): NOpt<V, V> {
    val vt: Tr<V> = {
        try {
            conversion(transformValue(it))
        } catch (e: NumberFormatException) {
            fail(e.message ?: "")
        }
    }
    return copy(vt, { it.lastOrNull() })
}
inline fun <InT : Any, V : Any> NOpt<InT, InT>.convert(name: String, crossinline conversion: Conv<InT, V>): NOpt<V, V> =
    convert(conversion)

// A call with named arguments, or another number of them, is the
// extension's, not an invoke of the property of its name.
fun <E, V> NOpt<E, V>.transformAll(n: Int = this.n, f: All<E, List<E>>): Opt<List<E>, E, V> = copy(transformValue, f, n)
fun <E : Any, V> NOpt<E, V>.default(d: E): Opt<E, E, V> = copy(transformValue, { transformAll(it) ?: d })
fun <E, V> NOpt<E, V>.multiple(): Opt<List<E>, E, V> = transformAll(n = -1) { it }

fun raw(): NOpt<String, String> = Impl({ it }, { it.lastOrNull() }, 1)

// A self-referencing bound: a star projection's capture is its bound.
abstract class Base<T : Base<T>>(val name: String) {
    var subs: List<T> = emptyList()
    open fun aliases(): Map<String, String> = emptyMap()
}
class Cmd(name: String) : Base<Cmd>(name) {
    override fun aliases() = mapOf("x" to name)
}
fun names(c: Base<*>): List<String> = c.subs.map { it.name }

// A property with the name of the member it calls.
class Parser<T : Base<T>>(private val command: T) {
    private val aliases = command.aliases()
    fun alias(k: String) = aliases[k]
}

// The expected type infers the left side of ?: and of !!.
class Data(val data: MutableMap<String, Any?>) {
    inline fun <reified T : Any> find(key: String): T? = data[key] as? T
    inline fun <reified T : Any> findOrSet(key: String, d: () -> T): T = find(key) ?: d().also { data[key] = it }
}
inline fun <reified T : Any> require(d: Data, key: String): ReadOnlyProperty<Any?, T> = ReadOnlyProperty { _, _ -> d.find(key)!! }
val data = Data(mutableMapOf("k" to "v", "n" to 3))
val required by require<String>(data, "k")

// A generic delegate's type argument is the property's type.
class Late<T>(private val msg: String) : ReadWriteProperty<Any, T> {
    private var v: Any? = null
    override fun getValue(thisRef: Any, property: KProperty<*>): T = v as T
    override fun setValue(thisRef: Any, property: KProperty<*>, value: T) {
        v = value
    }
}
class Holder<A> {
    var value: A by Late("x")
}

// provideDelegate registers the property.
class Registry {
    val names = mutableListOf<String>()
}
class Arg(val value: String) {
    operator fun provideDelegate(thisRef: Command, property: KProperty<*>): ReadOnlyProperty<Command, String> {
        thisRef.registry.names += property.name
        return ReadOnlyProperty { _, _ -> value }
    }
}
class Command {
    val registry = Registry()
    val shell by Arg("bash")
    val path by Arg("/tmp")
}

// Pair<String, Nothing?> and Pair<String, String> join as
// Pair<String, String?>.
fun split(tok: String): String {
    val i = tok.indexOf('=')
    var (name, value) = if (i >= 0) tok.substring(0, i) to tok.substring(i + 1) else tok to null
    name = name.uppercase()
    return name + ":" + (value?.length ?: -1)
}

// The standard library's hidden maxBy, which returned T?, isn't a
// candidate.
fun longest(names: Set<String>) = names.maxBy { it.length }

// A contract's returns(false) implies (this != null) smart-casts a member.
data class Row(val marker: String?)
fun prefix(row: Row) = when {
    row.marker.isNullOrEmpty() -> "-"
    else -> row.marker + row.marker.length
}

// Overloads with function types of definitely non-null parameters.
class Check<A>(val v: A)
fun <A> Check<A>.check(message: String, f: (A & Any) -> Boolean): String = check({ message }, f)
fun <A> Check<A>.check(lazyMessage: (A & Any) -> String, f: (A & Any) -> Boolean): String {
    val x = v ?: return "null"
    return if (f(x)) "ok" else lazyMessage(x)
}

// Builder inference: a call of a plain function, a local extension
// function of the receiver's type, and yieldAll's overloads.
fun style(s: String): String = "[$s]"
fun usage(n: Int): String = buildList {
    if (n > 0) {
        val metavar = "n$n"
        add(style(metavar))
    }
    add("end")
}.joinToString(" ")

class Node(val name: String, val subs: List<Node>)
fun flat(root: Node) = sequence {
    suspend fun SequenceScope<Node>.yieldSubs(n: Node) {
        yield(n)
        for (s in n.subs) yieldSubs(s)
    }
    yieldSubs(root)
}
fun both(seq: Sequence<Int>) = sequence { yieldAll(seq); yieldAll(listOf(9)) }
fun either(b: Boolean, seq: Sequence<Int>): List<Int> = (if (b) sequence { yield(1); yield(2) } else seq).toList()

// A local extension function called on an implicit receiver.
fun digits() = buildString {
    fun StringBuilder.add(x: Int) {
        append(x)
        if (x < 3) add(x + 1)
    }
    add(1)
}

// A lambda in a copy of an inline function sees the copy's type
// arguments.
inline fun <reified T> isA(): (Any) -> Boolean = { it is T }

fun main() {
    val ctx = Ctx("c")
    println(ctx.int("12") + ctx.long("30"))
    println(3.convert { "#$it" } + ", " + "x".convert { it.uppercase() })

    val o = raw().convert("x") { it.toInt() * 2 }
    println(o.transformAll(ctx, listOf(o.transformValue(ctx, "21"))))
    println(o.copy(n = 3).n)
    println(raw().default("d").transformAll(ctx, emptyList()))
    val m = raw().multiple()
    println("${m.n} ${m.transformAll(ctx, listOf("a", "b"))}")

    val c = Cmd("a")
    c.subs = listOf(Cmd("b"), Cmd("c"))
    println(names(c))
    println(Parser(c).alias("x"))

    println(data.findOrSet("k") { "w" } + data.findOrSet("z") { "q" } + data.find<Int>("n"))
    println(required)

    val h = Holder<Int>()
    h.value = 7
    println(h.value)
    val cmd = Command()
    println("${cmd.registry.names} ${cmd.shell} ${cmd.path}")

    println(split("a=bc") + " " + split("a"))
    println(longest(setOf("a", "abc", "ab")))
    println(prefix(Row(null)) + prefix(Row("")) + prefix(Row("xy")))
    println(Check<Int?>(3).check("small") { it > 5 } + " " + Check<Int?>(9).check("small") { it > 5 } + " " + Check<Int?>(null).check("small") { it > 5 })
    println(listOf(APPEND, WRITE).joinToString())

    println(usage(2) + " | " + usage(0))
    val t = Node("a", listOf(Node("b", listOf(Node("c", emptyList()))), Node("d", emptyList())))
    println(flat(t).map { it.name }.toList())
    println(both(sequenceOf(7, 8)).toList())
    println(either(true, sequenceOf(5)) + either(false, sequenceOf(5)))
    println(digits())
    println(isA<String>()("s").toString() + isA<String>()(1))
}

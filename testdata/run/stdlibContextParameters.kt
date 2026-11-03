// Context parameters, as Arrow's raise.context package and its
// Comparison.kt use them: top-level and extension functions with
// context parameters, named and _, which take them first on the JVM,
// before the extension receiver; context arguments from enclosing
// functions' contexts, lambdas' receivers, extension receivers and the
// class's this, the nearest first; context function types, which are
// function types with the contexts as their first parameters, called with
// implicit and explicit contexts, and lambdas of them; inline and
// suspend context functions; a private context operator; default values;
// safe calls; a context argument that is the class's this; the standard
// library's context(a) { }; contexts that local functions, object
// expressions and lambdas, suspend ones too, capture.

import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.startCoroutine

class Logger(val name: String) {
    val lines = mutableListOf<String>()
    fun log(s: String) { lines += "$name: $s" }
}

class Config(val verbose: Boolean)

interface Raise<in E> {
    fun raise(e: E): Nothing
}

class RaiseException(val e: Any?) : Exception()

class DefaultRaise<E> : Raise<E> {
    override fun raise(e: E): Nothing = throw RaiseException(e)
}

sealed class Either<out E, out A> {
    data class Left<E>(val e: E) : Either<E, Nothing>()
    data class Right<A>(val a: A) : Either<Nothing, A>()
}

// A builder whose block is a receiver lambda, as arrow.core.raise's.
inline fun <E, A> eitherExt(block: Raise<E>.() -> A): Either<E, A> =
    try {
        Either.Right(DefaultRaise<E>().block())
    } catch (r: RaiseException) {
        @Suppress("UNCHECKED_CAST")
        Either.Left(r.e as E)
    }

// Its contextual form, as arrow.core.raise.context's: the context lambda
// is passed where the receiver lambda is expected.
inline fun <E, A> either(block: context(Raise<E>) () -> A): Either<E, A> = eitherExt(block)

context(r: Raise<E>)
fun <E> raise(e: E): Nothing = r.raise(e)

context(r: Raise<E>)
inline fun <E> ensure(condition: Boolean, otherwise: () -> E) {
    if (!condition) r.raise(otherwise())
}

context(r: Raise<E>)
fun <E, A> Either<E, A>.bind(): A = when (this) {
    is Either.Left -> r.raise(e)
    is Either.Right -> a
}

context(logger: Logger)
fun info(s: String) = logger.log(s)

context(logger: Logger, config: Config)
fun debug(s: String) {
    if (config.verbose) logger.log("debug $s")
}

context(_: Logger)
fun twice(s: String) {
    info(s)
    info(s)
}

context(logger: Logger)
fun String.shout(times: Int = 1): String {
    val s = uppercase().repeat(times)
    logger.log(s)
    return s
}

context(logger: Logger)
fun Int.describe(): String = "int $this from ${logger.name}"

context(logger: Logger)
fun Long.describe(): String = "long $this from ${logger.name}"

context(c: Comparator<T>)
private operator fun <T> T.compareTo(other: T): Int = c.compare(this, other)

fun <A> max(a: A, b: A, comparator: Comparator<A>): A = with(comparator) { if (a >= b) a else b }

inline fun <A> logged(name: String, block: context(Logger) () -> A): Pair<A, List<String>> {
    val logger = Logger(name)
    val a = block(logger)
    return a to logger.lines
}

fun <A> loggedNotInline(name: String, block: context(Logger) () -> A): Pair<A, List<String>> {
    val logger = Logger(name)
    val a = with(logger) { block() }
    return a to logger.lines
}

context(logger: Logger)
inline fun <A> section(title: String, block: context(Logger) (String) -> A): A {
    logger.log("begin $title")
    val a = block(title.length.toString())
    logger.log("end $title")
    return a
}

// A context lambda passed where a receiver lambda is expected, and the
// other way around.
fun withLogger(block: Logger.() -> Unit): List<String> {
    val l = Logger("recv")
    l.block()
    return l.lines
}

fun contextual(block: context(Logger) () -> Unit): List<String> = withLogger(block)

val asContext: context(Logger) () -> Unit = { info("from a property") }

class Service(val logger: Logger) {
    // Logger.log needs a Logger: the nearest is the with's receiver.
    fun run(): List<String> {
        with(logger) { info("service") }
        with(Logger("inner")) { info("nearest") }
        return logger.lines
    }
}

fun Logger.viaExtension() = info("via the extension receiver")

suspend fun <A> suspended(a: A): A = a

context(logger: Logger)
suspend fun fetch(id: Int): String {
    val s = suspended("item $id")
    logger.log("fetched $s")
    return s
}

context(logger: Logger)
suspend fun fetchAll(n: Int = 2): String {
    var s = ""
    for (i in 1..n) s += fetch(i) + ";"
    return s
}

context(logger: Logger)
inline fun String.tagged(f: (String) -> String): String {
    logger.log("tag $this")
    return f(this)
}

fun withBoth(block: context(Logger) String.(Int) -> String): Pair<String, List<String>> {
    val logger = Logger("both")
    val s = with(logger) { "recv".block(3) }
    return s to logger.lines
}

open class Named(val name: String)

context(n: Named)
fun describeNamed() = "named ${n.name}"

class Widget : Named("widget") {
    fun describe() = describeNamed()
}

context(_: Logger)
fun captured(): Runnable {
    fun local(n: Int) = info("local $n")
    local(1)
    val f = { info("lambda") }
    f()
    val g: () -> Unit = fun() { info("anonymous") }
    g()
    return object : Runnable {
        override fun run() = info("object")
    }
}

context(_: Logger)
fun later(): suspend () -> String = { fetch(3) }

suspend fun <A> runLogged(name: String, block: suspend context(Logger) () -> A): Pair<A, List<String>> {
    val logger = Logger(name)
    val a = block(logger)
    return a to logger.lines
}

fun runBlocking(block: suspend () -> Unit) {
    block.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
}

fun parse(s: String): Either<String, Int> =
    s.toIntOrNull()?.let { Either.Right(it) } ?: Either.Left("not a number: $s")

context(_: Raise<String>)
fun sum(a: String, b: String): Int = parse(a).bind() + parse(b).bind()

fun main() {
    val logger = Logger("main")
    with(logger) {
        info("hello")
        twice("again")
        println("shout " + "hi".shout() + " " + "ho".shout(times = 2))
        println(3.describe())
        println(4L.describe())
        val s: String? = "maybe"
        println(s?.shout())
        val n: String? = null
        println(n?.shout())
    }
    with(Config(true)) {
        with(logger) { debug("both") }
    }
    with(logger) {
        with(Config(false)) { debug("quiet") }
    }
    println(logger.lines)

    println(either<String, Int> { sum("1", "2") })
    println(either<String, Int> { sum("1", "x") })
    println(either { ensure(false) { "ensured" }; 1 })
    println(either<String, Int> { raise("raised") })
    println(either<String, String> { Either.Right("right").bind() })

    println(max(1, 2, naturalOrder()))
    println(max("b", "a", naturalOrder()))
    println(max(1, 2, reverseOrder()))

    println(logged("inline") { info("one"); 1 })
    println(loggedNotInline("not inline") { twice("two"); 2 })
    println(logged("section") {
        section("title") { len -> info("length $len"); len }
    })
    println(contextual { info("context to receiver") })
    println(withLogger(asContext))
    println(contextual(asContext))
    with(Logger("explicit")) {
        asContext()
        asContext(Logger("argument"))
        println(lines)
    }
    println(Service(Logger("service")).run())
    with(Logger("safe")) {
        val s: String? = "s"
        val none: String? = null
        println(s?.tagged { it + "!" })
        println(none?.tagged { it + "!" })
        println(lines)
    }
    println(withBoth { n -> info("in $n"); this + n })
    println(Widget().describe())
    val cap = Logger("captured")
    with(cap) { captured() }.run()
    println(cap.lines)
    context(Logger("stdlib"), Config(true)) {
        debug("from context()")
        println(context(Named("inner")) { describeNamed() })
    }
    val ext = Logger("ext")
    ext.viaExtension()
    println(ext.lines)

    runBlocking {
        val logger = Logger("suspend")
        with(logger) { println(fetch(1)) }
        println(logger.lines)
        println(runLogged("suspend lambda") { fetch(2) })
        println(runLogged("defaults") { fetchAll() + fetchAll(1) })
        val l = Logger("later")
        val f = with(l) { later() }
        println(f() + " " + l.lines)
    }
}

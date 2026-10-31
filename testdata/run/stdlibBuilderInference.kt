// Builder inference: a lambda's receiver or parameter type with a type
// variable that nothing else gives is inferred from how the lambda's
// body uses its values.
class Buildee<T> {
    private val items = mutableListOf<T>()
    fun yield(arg: T) { items += arg }
    fun materialize(): T = items.first()
    override fun toString() = "Buildee$items"
}

fun <T> build(instructions: Buildee<T>.() -> Unit): Buildee<T> = Buildee<T>().apply(instructions)

fun <T> buildWith(instructions: (Buildee<T>) -> Unit): Buildee<T> = Buildee<T>().also(instructions)

class Target(name: String) : Base(name) {
    fun renamed(n: String) = Target(n)
    override fun toString() = "Target($name)"
}

fun consume(t: Target) = println("consumed $t")

fun sink(t: Target) = println("sank $t")

fun <T> share(from: T, to: T) {}

val Buildee<Target>.firstName: String get() = materialize().name

fun <T> run(instructions: (Buildee<T>) -> String): String = instructions(Buildee())

fun id(s: String) = s

class Pipe<T> {
    var producer: () -> T = { null!! }
    var consumer: (T) -> Unit = {}
    fun produce(p: () -> T) { producer = p }
    fun consume(c: (T) -> Unit) { consumer = c }
}

fun <T> pipe(instructions: Pipe<T>.() -> Unit): Pipe<T> = Pipe<T>().apply(instructions)

fun <T> parallel(a: Buildee<T>.(T) -> Unit, b: Buildee<T>.(T) -> Unit): Buildee<T> {
    @Suppress("UNCHECKED_CAST")
    val value = Target("m") as T
    return Buildee<T>().apply { a(value); b(value); yield(value) }
}

fun named(t: Base) = t.name

open class Base(val name: String)

class Holder<T : Any>(val buildee: Buildee<T>)

fun consumeAny(b: Buildee<Any>) = println("any $b")

interface Scope<S> {
    fun <B> result(p: Provider<S, B>): B
}

interface Provider<A, B> {
    fun provide(): A
}

fun <A, B> provider(block: Scope<A>.() -> B): Provider<A, B> = object : Provider<A, B> {
    @Suppress("UNCHECKED_CAST")
    override fun provide(): A = "o" as A
}

val anyProvider: Provider<Any, Any> = object : Provider<Any, Any> {
    override fun provide(): Any = "p"
}

class Bounded<T : Base> {
    var last: Any? = null
    fun set(value: T) { last = value }
}

fun Bounded<Target>.set(other: Int) { last = other }

fun <T : Base> bounded(instructions: Bounded<T>.() -> Unit): Bounded<T> = Bounded<T>().apply(instructions)

class Relay<A, B> {
    private var a: A? = null
    var b: B? = null
    fun emit1(x: A) { a = x }
    fun get(): A = a!!
    fun emit2(x: B) { b = x }
}

fun <A, B> relay(block: Relay<A, B>.() -> Unit): Relay<A, B> = Relay<A, B>().apply(block)

class Step<O, F>(val log: String) {
    fun group(): Step<F, String> = Step("$log.group")
    fun <R> map(from: Step<O, *>, f: (F) -> R): Step<O, R> = Step("$log.map")
}

fun <T> chain(a: (Step<T, T>) -> Step<T, T>): String = a(Step("start")).log

class Wrapped(val s: String)

class Source<A, B>

fun <Y : Any> source(): Source<Y, Target> = Source()

class Owner<T> {
    fun <X> constrain(s: Source<X, T>): Source<X, T> = s
}

fun <OT> owned(block: Owner<OT>.() -> Source<*, OT>): String = "owned"

class Cell<T> {
    @Suppress("UNCHECKED_CAST")
    fun take(): T = Unit as T
}

fun <T> cell(block: Cell<T>.() -> Unit): Cell<T> = Cell<T>().apply(block)

class Slot<T> {
    var value: Any? = null
    fun resumeWith(r: Result<T>) { value = r }
}

fun <T> slot(block: (Slot<T>) -> Unit): Slot<T> = Slot<T>().also(block)

fun <T> failed(message: String): Result<T> = Result.failure(IllegalStateException(message))

fun <E> Buildee<out E>.offer(value: E): E = value

fun <T> pick(vararg values: T): T = values.last()

fun take(value: Any) = println("any $value")
fun take(value: Wrapped) = println("wrapped ${value.s}")

fun <T> pickBuildee(vararg values: Buildee<T>): Buildee<T> = values.first()

class Tree(val name: String, val children: List<Tree>)

class Cont<T> {
    var value: Any? = null
    fun resume(x: T, onCancel: ((String) -> Unit)?) { value = x; onCancel?.invoke("x") }
}

@Suppress("UNCHECKED_CAST")
inline fun <T> suspendish(block: (Cont<T>) -> Unit): T { val c = Cont<T>(); block(c); return c.value as T }

inline fun <R> retrieve(onElement: (String) -> R): R = onElement("j")

// A function whose type is inferred while the lambda is checked to infer
// T: its reference, bound to this, is checked once.
class Channel<E>(val tag: String, val handler: E?) {
    fun receive() = suspendish { cont ->
        retrieve { element ->
            val onCancel = handler?.bind()
            cont.resume(element, onCancel)
        }
    }
    private fun cancelled(x: String) = println("$tag cancelled $x")
    private fun E.bind() = ::cancelled
}

fun offered0() = false

interface Outbox<in E> {
    val onSend: Offer<E, Outbox<E>>
}

class Offer<in P, out Q>(val target: Q)

operator fun <P, Q> Offer<P, Q>.invoke(param: P, block: (Q) -> Unit) {
    println("offer $param")
    block(target)
}

fun <E> outbox(block: Outbox<E>.() -> Unit): String {
    val o = object : Outbox<E> {
        override val onSend = Offer<E, Outbox<E>>(this)
        override fun toString() = "box"
    }
    o.block()
    return "outbox"
}

open class Store<T> {
    @Suppress("UNCHECKED_CAST")
    fun take(): T = Unit as T
}

class SubStore<T> : Store<T>()

fun <T> store(block: Store<T>.() -> Unit): Store<T> = SubStore<T>().apply(block)

fun main() {
    // A value passed where the variable is expected.
    val target = Target("a")
    val yielded = build { yield(target) }
    println(yielded)

    // A value of the variable passed where another type is expected.
    val materialized = build {
        yield(Target("b"))
        consume(materialize())
    }
    println(materialized)
    val consumed = build { if (materialized.toString().isEmpty()) consume(materialize()) }
    println(consumed)

    // A variable of a call in the lambda that both share.
    val shared = build {
        yield(1)
        share(2, materialize())
    }
    println(shared.materialize() + 1)

    // The receiver assigned to a variable of a known type.
    val assigned = build {
        val typed: Buildee<String> = this
        typed.yield("c")
    }
    println(assigned.materialize().length)

    // An extension property of the receiver.
    val extended = build {
        yield(Target("d"))
        println(firstName)
    }
    println(extended)

    // A parameter, rather than the receiver.
    val parameter = buildWith { it.yield(Target("e")) }
    println(parameter)

    // The lambda's result type.
    println(run { b -> b.yield("f"); b.materialize() })

    // An explicitly typed parameter.
    val explicit = buildWith { b: Buildee<Long> -> b.yield(7L) }
    println(explicit.materialize() * 2)

    // A lambda in the lambda that captures one of its locals.
    val captured = build {
        val x = id("g")
        val get = { x + "!" }
        yield(get())
    }
    println(captured)

    // A local class whose member gives the type.
    val local = build outer@{
        class Local {
            fun give() { this@outer.yield(Target("h")) }
        }
        Local().give()
    }
    println(local)

    // A local class as the type, and an object expression's.
    val ofLocal = build {
        class Named(val n: Int) { override fun toString() = "Named($n)" }
        yield(Named(1))
    }
    println(ofLocal.materialize().n + 1)
    val ofObject = build {
        yield(object { val n = 2; override fun toString() = "object($n)" })
    }
    println(ofObject.materialize().n + 1)

    println(Channel("i", "h").receive())

    // A value of the variable's type as a receiver: what is known of the
    // variable so far fixes it, and the receiver has its members.
    val p = pipe {
        produce { Target("k") }
        consume { println(it.name + " " + it.renamed("l")) }
    }
    p.consumer(p.producer())

    // Two lambdas of one call over the variable: both are checked before
    // it is fixed, and it takes both bounds.
    val both = parallel({ println(named(it)) }, { consume(it) })
    println(both.materialize().name)

    // Builders in the branches of an if or when statement: one without
    // constraints takes the others' type.
    if (both.toString().isNotEmpty()) build { yield(Target("o")) } else build {}
    when (both.toString()) {
        "" -> build { yield(Target("o")) }
        "-" -> build {}
        else -> Buildee()
    }

    // A variable that is another call's with a declared bound is that
    // bound, whatever the lambda gives.
    val held = Holder(build { yield(Target("q")) })
    consumeAny(held.buildee)

    // Nested builders whose calls have their own type parameters.
    println(provider { result(provider { result(anyProvider) }) }.provide())

    // An argument that the variable's bound rules out: an extension, not
    // the member, takes it.
    println(bounded { set(Target("r")); set(1) }.last)

    // A value of one variable passed for another: the second takes the
    // first's type.
    println(relay { emit1(Target("s")); emit2(get()) }.b)

    // A destructured parameter's declared types.
    val typed = buildMap { mapValues { (key: Int, value: String) -> "$key$value" } }
    println(typed.isEmpty())

    // The lambda's result, whose type has the variable.
    println(chain { it.group().map(it, ::Wrapped) })

    // A variable equal to another with a declared bound is below it.
    println(owned { constrain(source()) })

    // A lambda or a reference assigned to the builder's property of a
    // function type with the variable.
    pipe { consumer = { println(named(it)) } }.consumer(Target("t"))
    pipe { consumer = {}; consumer = ::sink }.consumer(Target("u"))

    // An anonymous function takes the builder as its parameter.
    println(build(fun(b) { b.yield(Target("v")) }))

    // A value returned from a Unit lambda is a Unit.
    println(cell { return@cell take() }.take())

    // Only a Nothing: the variable is Nothing.
    val nothing = build {
        try {
            yield(throw IllegalStateException("w"))
        } catch (e: IllegalStateException) {
            println("caught ${e.message}")
        }
    }
    val exact: Buildee<Nothing> = nothing
    println(exact)

    // A parameter's variable, and a call's below it: the call's is the
    // parameter's.
    val slotted = slot { s ->
        s.resumeWith(Result.success("x"))
        s.resumeWith(failed("y"))
    }
    val ofSlot: Slot<String> = slotted
    println(ofSlot.value)

    // A generic call in the lambda is inferred with the builder's
    // variable: offer's E is above both Target and String, and the
    // branches' join above both Target and Wrapped.
    val offered = build {
        yield(Target("x"))
        println(offer("y"))
        println(pick(materialize(), Wrapped("z")) is Base)
        println((if (offered0()) materialize() else Wrapped("z")) is Base)
    }
    val ofTarget: Buildee<Target> = offered
    println(ofTarget)

    // A candidate whose constraint contradicts the earlier ones isn't
    // applicable: take(Any), not take(Wrapped).
    build { yield(Target("a1")); take(materialize()) }

    // Builders whose variables only another call's bounds.
    println(pickBuildee(build { yield(Target("b1")) }, build {}))

    // A lambda's parameter types that are known are fixed first.
    println(listOf(1, 2).flatMap { n -> buildList { if (n > 1) add(n) } })

    // A local extension of the builder's type in its lambda.
    val trees = sequence {
        suspend fun SequenceScope<Tree>.walk(t: Tree) {
            yield(t)
            for (c in t.children) walk(c)
        }
        walk(Tree("r", listOf(Tree("c", listOf()))))
    }
    println(trees.map { it.name }.toList())

    // A property invoked by an invoke operator: checked afresh after the
    // variable is fixed.
    println(outbox { onSend("c1") { println("sent to $it") } })

    // A lambda returning Unit that ends with a value of the variable's
    // type: the variable is Unit, unless its bounds rule that out. A cast
    // of the builder to a subclass with * keeps the variable.
    val stored = store { take() }
    val ofStore: Unit = stored.take()
    println(ofStore)
    val cast = store {
        this as SubStore<*>
        take()
    }
    val ofCast: Unit = cast.take()
    println(ofCast)
    val nested = store { listOf(1).forEach { take() } }
    val ofNested: Unit = nested.take()
    println(ofNested)
    println(buildList { add(1); add(2); removeAt(0) })
}

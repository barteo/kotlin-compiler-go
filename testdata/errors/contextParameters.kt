// The parser's errors in context parameters are in internal/syntax's
// TestParseErrors and TestMisplacedContexts.
class Logger(val name: String)

context(logger: Logger)
fun info(s: String) {}

fun noLogger() = info("x") // ERROR no context argument for 'logger: Logger' found

fun reference() {
    val r = ::info // ERROR no context argument for 'logger: Logger' found
}

context(logger: Logger)
fun referenceInContext() {
    val r = ::info // ERROR callable reference to 'info' is unsupported because it has context parameters
    val h = Holder::p // ERROR callable reference to 'p' is unsupported because it has context parameters
}

context(a: Logger, a: Logger) // ERROR conflicting declarations: a
fun twice() {}

fun withLogger(block: context(Logger) () -> Unit) {
    block() // ERROR no context argument for 'Logger' found
    block(Logger("explicit"))
}

context(a: Logger) // ERROR property with context parameters cannot be initialized because it has no backing field
val initialized: String = "x"

context(a: Logger) // ERROR property with context parameters cannot be initialized because it has no backing field
val noGetter: String // ERROR property must be initialized

context(a: Logger)
var withAccessors: String = "x" // ERROR initializer is not allowed here because this property has no backing field
    get() = a.name
    set(value) {}

val anonymous: context(Logger) () -> Unit = fun () {} // ERROR the anonymous function's context parameters don't match the expected function type

class Holder {
    context(a: Logger)
    val p: String get() = a.name
}

fun noContext() = Holder().p // ERROR no context argument for 'a: Logger' found

context(a: Logger)
val twin: String get() = a.name

val twin: String get() = "plain" // ERROR conflicting declarations: twin

val twin: Int get() = 1 // ERROR conflicting declarations: twin

context(l: Logger)
fun bothTwins() = twin // ERROR overload resolution ambiguity: twin

context(a: Logger, b: Logger)
fun twoLoggers() = info("x") // ERROR multiple potential context arguments for 'Logger' in scope

interface Shape {
    context(a: Logger)
    fun area(): Int
}

class Square : Shape { // ERROR class 'Square' is not abstract and does not implement abstract member 'area'
    override fun area() = 1 // ERROR 'area' overrides nothing
}

abstract class Fn : context(Logger) () -> Unit // ERROR extension function type or function type with context parameters is not allowed as supertype

context(a: Logger, b: Holder)
val ordered: String get() = "" // ERROR conflicting declarations: ordered

context(b: Holder, a: Logger)
val ordered: String get() = "" // ERROR conflicting declarations: ordered

context(l: Logger) // ERROR context parameters on classes are unsupported
class WithContext {
    fun f() = l.name // ERROR unresolved reference: l
}

class Ctor {
    context(l: Logger) // ERROR context parameters on constructors are unsupported
    constructor()
}

context(f: () -> Unit) // ERROR context parameter 'f: () -> Unit' must be 'noinline'
inline fun inlineContext() {}

context(noinline f: () -> Unit)
inline fun noinlineContext() {}

class Named(val name: String) {
    fun hello() = name
}

context(n: Named)
fun Named.shadowed() = hello() // ERROR 'hello' uses an implicit receiver shadowed by a context parameter

context(n: Named)
fun Named.explicit() = this.hello() + n.hello()

fun withNamed(block: context(Named) () -> Unit) = block(Named("c"))

fun Named.nested() {
    withNamed {
        hello() // ERROR 'hello' uses an implicit receiver shadowed by a context parameter
        name // ERROR 'name' uses an implicit receiver shadowed by a context parameter
    }
}

fun <T> on(t: T, block: T.() -> Unit) = t.block()

context(n: Named)
fun inner() {
    on(Named("w")) { hello() } // the receiver is nearer
    fun Named.local() = hello() // the local function's receiver is nearer
}

class CtxA
interface CtxFirst {
    context(a: CtxA)
    val cb: String
}
interface CtxSecond {
    context(a: Any)
    val cb: String
}
interface CtxBoth : CtxFirst, CtxSecond

context(x: CtxA)
fun bothContexts(a: CtxBoth) {
    a.cb // ERROR overload resolution ambiguity: cb
}

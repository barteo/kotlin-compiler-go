// The parser's unsupported uses of context parameters are in
// internal/syntax's TestUnsupportedFeatures.
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

context(a: Logger)
val initialized: String = "x" // ERROR initializer is not allowed here because this property has no backing field

context(a: Logger)
val noGetter: String // ERROR a property with context parameters must have accessors or be abstract

val anonymous: context(Logger) () -> Unit = fun () {} // ERROR the anonymous function's context parameters don't match the expected function type

class Holder {
    context(a: Logger)
    val p: String get() = a.name
}

fun noContext() = Holder().p // ERROR no context argument for 'a: Logger' found

context(a: Logger)
val twin: String get() = a.name

val twin: String get() = "plain"

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
val ordered: String get() = ""

context(b: Holder, a: Logger)
val ordered: String get() = "" // ERROR conflicting declarations: ordered

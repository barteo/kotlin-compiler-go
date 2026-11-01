// The parser's unsupported uses of context parameters are in
// internal/syntax's TestUnsupportedFeatures.
class Logger(val name: String)

context(logger: Logger)
fun info(s: String) {}

fun noLogger() = info("x") // ERROR no context argument for 'logger: Logger' found

fun reference() {
    val r = ::info // ERROR callable references to functions with context parameters are not supported
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

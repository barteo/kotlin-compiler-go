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

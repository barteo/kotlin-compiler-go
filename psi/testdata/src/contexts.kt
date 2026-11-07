// Context parameters (Kotlin 2.4 PSI).
interface Logger { fun log(s: String) }
class Config(val name: String)

context(logger: Logger)
fun info(msg: String) = logger.log(msg)

context(logger: Logger, config: Config)
fun both() {}

context(_: Logger)
val greeting: String get() = "hi"

context(logger: Logger) @Suppress("x") public fun annotated() {}

@Suppress("x") context(c: Config) private fun afterAnnotation() {}

context(c: Config)
class Holder {
    context(logger: Logger)
    fun member() {}

    context(logger: Logger)
    val prop: Int get() = 1
}

// Deprecated context receivers.
context(Logger)
fun receiverForm() {}

context(Logger, label@Config)
fun receiverLabel() {}

// Empty and broken lists.
context()
fun empty() {}

context(a: Logger,)
fun trailingComma() {}

context(a: Logger b: Config)
fun missingComma() {}

// Function types.
val f: context(Logger) () -> Unit = {}
val g: context(logger: Logger, Config) (Int) -> String = { "" }
val h: suspend context(Logger) () -> Unit = {}

fun locals() {
    context(logger: Logger)
    fun local() {}

    context(logger: Logger) fun local2() = 1

    val anon = context(logger: Logger) fun() {}
    context(x)
    context(1, 2)
    val notContext = context
}

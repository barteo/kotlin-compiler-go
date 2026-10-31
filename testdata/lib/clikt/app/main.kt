import lib.*
import lib.Context.Companion.TERMINAL_KEY

class App : Command("app") {
    val shell by option("bash")
    val path by option("/tmp")
}

class Plain : Formatter<Int>() {
    override fun sections(): List<Section<Int>> = listOf(Section("a", 1), Section("b", 2))
}

interface Converter<T> {
    fun convert(input: String): T
}

fun <T> Transforms<T>.converter(): Converter<T> = object : Converter<T> {
    override fun convert(input: String): T = transformEach(listOf(transformValue(input)))
}

val Context.terminal: String
    get() = findObject(TERMINAL_KEY) ?: "none"

fun main() {
    val root = Context(null, mutableMapOf("obj" to "root", TERMINAL_KEY to "tty", "n" to 1))
    val child = Context(root)
    println(child.findObject<String>())
    println(child.findObject<Int>("n"))
    println(child.findObject<Int>())
    println(child.terminal + " " + Context(null).terminal)
    val req by child.requireObject<String>()
    println(req)

    val app = App().versionOption("1.0").versionOption("2.0") { "v$it!" }
    app.actions.forEach { it() }
    println("${app.registered} ${app.shell} ${app.path}")
    val conv = app.convert { it.toInt() * 2 }
    println(conv(" 21 "))
    println(app.convert({ "N" }) { it.length }("abc"))

    println(Plain().render())

    val t = object : Transforms<Int> {
        override val transformValue: (String) -> Int = { it.toInt() }
        override val transformEach: (List<Int>) -> Int = { it.sum() + 1 }
    }
    println(t.converter().convert("41"))
}

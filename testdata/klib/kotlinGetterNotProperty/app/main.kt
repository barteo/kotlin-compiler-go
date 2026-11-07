import h.Repo

class Test {
    private val enabled = "outer property"
    private val url = "outer url"

    fun run() = with(Repo()) { "$enabled, $url, $name, ${getEnabled()}" }
}

class Sub : Repo() {
    fun describe() = "${getEnabled()} ${getURL()} $name"
}

fun main() {
    println(Test().run())
    println(Sub().describe())
}

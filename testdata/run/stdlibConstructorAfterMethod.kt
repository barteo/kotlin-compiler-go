class Settings(val mode: Int, val all: Boolean)

class Scanner {
    private fun Settings() = Settings(mode = 2, all = false)

    fun settings() = Settings()
}

fun main() {
    val s = Scanner().settings()
    println("${s.mode} ${s.all}")
}

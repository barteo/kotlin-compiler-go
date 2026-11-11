abstract class Adapter {
    abstract fun bind(holder: Any, position: Int)
}

class Holder {
    fun bind(model: String) {
        println("bind $model")
    }
}

fun log(s: String) {
    println("log $s")
}

class MyAdapter : Adapter() {
    private fun item(p: Int): Any = if (p == 0) "header" else p

    override fun bind(holder: Any, position: Int) =
        when (holder) {
            is Holder -> (item(position) as? String)?.let { holder.bind(it) } ?: log("not a header")
            else -> log("unknown")
        }
}

fun orLog(x: String?) = x?.let { log(it) } ?: log("none")

fun main() {
    MyAdapter().bind(Holder(), 0)
    MyAdapter().bind(Holder(), 1)
    MyAdapter().bind(1, 0)
    val u: Unit = orLog(null)
    orLog("a")
    println(u)
}

import lib.*
import kotlin.coroutines.*

class AppRepo : Repo {
    override fun get(refresh: Boolean, clear: Boolean) = "app $refresh $clear"

    override suspend fun start(kind: String, ref: String?) = "app start $kind $ref"
}

class AppLeaf : Middle() {
    override fun greet(name: String, punct: String) = "sup $name$punct"
}

fun run(block: suspend () -> Unit) {
    block.startCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() })
}

fun main() {
    val d = DefaultRepo()
    println(d.get(refresh = true))
    println(d.get(false, true))
    val r: Repo = d
    println(r.get(true))
    println(AppRepo().get(refresh = false))
    println(listOf(Middle().greet("a"), Leaf().greet("b"), Leaf().greet("c", "?"), AppLeaf().greet("d")))
    println(Names().put("ab"))
    println(Names().put("ab", times = 3))
    run {
        println(d.start("sipp"))
        println(AppRepo().start("isa", "x"))
        println(AppRepo().start("lisa"))
    }
}

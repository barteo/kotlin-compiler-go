import kotlin.coroutines.Continuation
import kotlin.coroutines.EmptyCoroutineContext
import kotlin.coroutines.createCoroutine
import kotlin.coroutines.resume
import lib.Base

fun launch(block: suspend () -> Unit) {
    block.createCoroutine(Continuation(EmptyCoroutineContext) { it.getOrThrow() }).resume(Unit)
}

class Model : Base() {
    override var mode = 1

    override fun describe() = "model"

    fun load() {
        // A suspend lambda: a class of its own.
        launch {
            emit("loaded")
            state = "busy"
            val n = emitLater(state)
            emit("n=$n count=$count")
            emit(pick("a", "b"))
            emit("x".tagged() + "abc".size2)
            mode += 1
            emit("mode=$mode " + super.describe() + " " + describe())
        }
    }

    fun later(): () -> Unit = {
        // An ordinary lambda.
        emit("lambda $state")
        state = "done"
    }

    fun obj(): Runnable = object : Runnable {
        override fun run() {
            emit("object $count")
        }
    }

    inner class Inner {
        fun go() {
            emit("inner")
            this@Model.state = "inner"
        }
    }

    fun local() {
        fun f() = emit("local ${helper()} $LIMIT")
        f()
        listOf(1, 2).forEach { emit("each $it") }
        val r = Runnable { emit("sam ${pick(1, 2)}") }
        r.run()
    }
}

fun main() {
    val m = Model()
    m.load()
    m.later()()
    m.obj().run()
    m.Inner().go()
    m.local()
    println(m.dump())
}

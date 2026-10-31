import kotlinx.coroutines.*

// Set once the coroutine that is cancelled is working: main waits for it,
// not for a delay, which a loaded machine can outlast.
val working = CompletableDeferred<Unit>()

suspend fun compute(x: Int): Int {
    delay(10)
    return x * 2
}

suspend fun sumConcurrently(): Int = coroutineScope {
    val a = async { compute(1) }
    val b = async { compute(2) }
    a.await() + b.await()
}

fun main() = runBlocking {
    println("start")
    val job = launch {
        delay(20)
        println("launched")
    }
    println("before join")
    job.join()
    println(sumConcurrently())
    val r = withContext(Dispatchers.Default) {
        compute(21)
    }
    println(r)
    val cancelled = launch {
        try {
            repeat(1000) { i ->
                delay(5)
                if (i == 2) {
                    println("working $i")
                    working.complete(Unit)
                }
            }
        } finally {
            println("finally of cancelled")
        }
    }
    working.await()
    cancelled.cancelAndJoin()
    println("cancelled " + cancelled.isCancelled)
    val t = withTimeoutOrNull(50) {
        delay(1000)
        "late"
    }
    println(t)
    try {
        withTimeout(20) { delay(1000) }
    } catch (e: TimeoutCancellationException) {
        println("timed out")
    }
    val results = supervisorScope {
        val ok = async { compute(5) }
        val failing = async<Int> { throw IllegalStateException("boom") }
        val f = try {
            failing.await()
        } catch (e: IllegalStateException) {
            -1
        }
        ok.await() + f
    }
    println(results)
    try {
        coroutineScope {
            launch { throw IllegalArgumentException("child failed") }
            delay(100)
        }
    } catch (e: IllegalArgumentException) {
        println("caught " + e.message)
    }
    println("end")
}

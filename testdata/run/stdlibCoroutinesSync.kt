import kotlinx.coroutines.*
import kotlinx.coroutines.sync.*
import kotlin.coroutines.resume

class Account {
    private val mutex = Mutex()
    var balance = 0

    suspend fun deposit(n: Int) {
        mutex.withLock {
            val b = balance
            yield()
            balance = b + n
        }
    }
}

suspend fun waitFor(value: Int, cancelled: MutableList<String>): Int = suspendCancellableCoroutine { cont ->
    cont.invokeOnCancellation { cancelled.add("cancelled $value") }
    if (value >= 0) {
        cont.resume(value)
    }
}

fun main() = runBlocking {
    val account = Account()
    val jobs = List(10) { i ->
        launch { account.deposit(i) }
    }
    jobs.forEach { it.join() }
    println(account.balance)
    val cancelled = ArrayList<String>()
    println(waitFor(7, cancelled))
    val pending = launch { waitFor(-1, cancelled) }
    yield()
    pending.cancelAndJoin()
    println(cancelled)
    val sem = Semaphore(2)
    var maxRunning = 0
    var running = 0
    val workers = List(5) {
        launch {
            sem.withPermit {
                running++
                maxRunning = maxOf(maxRunning, running)
                delay(5)
                running--
            }
        }
    }
    workers.joinAll()
    println(maxRunning)
    val order = ArrayList<Int>()
    val a = launch {
        repeat(3) {
            order.add(it)
            yield()
        }
    }
    val b = launch {
        repeat(3) {
            order.add(10 + it)
            yield()
        }
    }
    joinAll(a, b)
    println(order)
}

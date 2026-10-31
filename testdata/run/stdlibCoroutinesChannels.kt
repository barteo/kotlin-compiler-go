import kotlinx.coroutines.*
import kotlinx.coroutines.channels.*

fun CoroutineScope.numbers(n: Int): ReceiveChannel<Int> = produce {
    for (i in 1..n) {
        send(i)
    }
}

fun CoroutineScope.squares(input: ReceiveChannel<Int>): ReceiveChannel<Int> = produce {
    for (x in input) {
        send(x * x)
    }
}

fun main() = runBlocking {
    val ch = Channel<String>()
    launch {
        for (w in listOf("a", "b", "c")) {
            ch.send(w)
        }
        ch.close()
    }
    for (w in ch) {
        println("got $w")
    }
    val sq = squares(numbers(5))
    var total = 0
    for (x in sq) {
        total += x
    }
    println(total)
    val buffered = Channel<Int>(capacity = 3)
    buffered.send(1)
    buffered.send(2)
    println(buffered.receive() + buffered.receive())
    println(buffered.tryReceive().isFailure)
    val results = Channel<Int>(Channel.UNLIMITED)
    val workers = List(3) { id ->
        launch {
            results.send(id * 10)
        }
    }
    workers.forEach { it.join() }
    results.close()
    val got = ArrayList<Int>()
    for (r in results) {
        got.add(r)
    }
    println(got.sorted())
    val rendezvous = Channel<Int>()
    val consumer = async {
        var sum = 0
        repeat(4) { sum += rendezvous.receive() }
        sum
    }
    repeat(4) { rendezvous.send(it + 1) }
    println(consumer.await())
}

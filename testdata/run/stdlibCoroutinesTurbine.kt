// What cashapp/turbine needs of kc, with kotlinx.coroutines: members of
// library companions imported, delegation to a library interface with
// hidden members, select clauses, value classes returned by suspend
// functions, locals read in a catch after a suspension, and inline suspend
// functions whose bodies take the caller's continuation.
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlinx.coroutines.*
import kotlinx.coroutines.channels.*
import kotlinx.coroutines.channels.Channel.Factory.UNLIMITED
import kotlinx.coroutines.selects.select

val defaultTimeout: Duration = 3000.milliseconds

// A channel that counts the values taken from it, delegating the rest:
// also cancel(Throwable) and cancel(), which are hidden.
class Counting<T>(private val channel: Channel<T>) : Channel<T> by channel {
  var taken = 0
  override fun tryReceive(): ChannelResult<T> {
    val result = channel.tryReceive()
    if (result.isSuccess) taken++
    return result
  }
}

// The first of a value and a job, as Turbine's wall-clock timeout races
// them.
suspend fun race(value: Deferred<Int>, timeout: Job): Int = select {
  value.onAwait { it * 2 }
  timeout.onJoin { -1 }
}

@JvmInline value class Name(val s: String)

suspend fun slowName(): Name {
  delay(1)
  return Name("slow")
}

suspend fun quickName(): Name = Name("quick")

suspend fun describe(name: String?, c: ReceiveChannel<Int>): String {
  val n = 3
  return try {
    withTimeout(10) { c.receive().toString() }
  } catch (e: TimeoutCancellationException) {
    "no value for $name in ${n}0ms"
  }
}

class Context(val name: String)

suspend fun named(): String = currentCoroutineContext()[CoroutineName]?.name ?: "?"

suspend fun drain(c: Channel<Int>): List<Int> {
  val items = buildList {
    while (true) {
      val item = c.tryReceive().getOrNull() ?: break
      add(item)
    }
  }
  return items
}

fun main() = runBlocking {
  println(defaultTimeout)
  println(1.5.seconds + 250.milliseconds)
  println(UNLIMITED == Channel.UNLIMITED)

  val counting = Counting(Channel<Int>(UNLIMITED))
  counting.send(1)
  counting.send(2)
  println(counting.tryReceive().getOrNull())
  println(counting.taken)
  counting.cancel()
  println(counting.isClosedForReceive)

  val value = async { delay(10); 21 }
  val never = launch { delay(10_000) }
  println(race(value, never))
  never.cancel()
  val slow = async { delay(10_000); 0 }
  println(race(slow, launch { }))
  slow.cancel()

  println(slowName().s + " " + quickName().s)
  val results = Channel<String>(1)
  launch { delay(5); results.send("late") }
  println(results.receiveCatching().getOrThrow())
  results.close()
  println(results.receiveCatching().isClosed)

  println(describe("never", Channel()))
  println(describe(null, Channel()))
  val ready = Channel<Int>(1)
  ready.send(7)
  println(describe("ready", ready))

  println(withContext(CoroutineName("ctx")) { Context(named()).name })

  val queued = Channel<Int>(UNLIMITED)
  for (i in 1..3) queued.send(i)
  println(drain(queued))
}

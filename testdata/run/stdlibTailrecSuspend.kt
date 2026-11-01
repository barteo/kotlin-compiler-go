// Tail calls of suspend tailrec functions, which loop as kotlinc's do
// instead of overflowing the stack: with no other suspension points, and
// in a state machine whose function suspends before its tail call.

import kotlin.coroutines.*

var pending: Continuation<Unit>? = null
var pauses = 0

suspend fun pause() = suspendCoroutine<Unit> { pauses++; pending = it }

suspend fun get(n: Int): Long {
  pause()
  return n.toLong()
}

// Runs block, resuming it from here each time it pauses.
fun <T> drive(block: suspend () -> T): T {
  var r: Result<T>? = null
  block.startCoroutine(Continuation(EmptyCoroutineContext) { r = it })
  while (r == null) {
    val c = pending!!
    pending = null
    c.resume(Unit)
  }
  return r!!.getOrThrow()
}

tailrec suspend fun sum(n: Long, acc: Long = 0): Long = if (n == 0L) acc else sum(n - 1, acc + n)

tailrec suspend infix fun Int.down(acc: Int): Int = if (this == 0) acc else (this - 1) down (acc + 1)

tailrec suspend fun count(x: Any, n: Int): Int = when (x) {
  is Int -> if (x == 0) n else (count(x - 1, n + 1))
  is String -> count(x.length, n)
  else -> n
}

tailrec suspend fun <T, A> Iterator<T>.foldl(acc: A, f: suspend (A, T) -> A): A =
  if (!hasNext()) acc else foldl(f(acc, next()), f)

var steps = 0

tailrec suspend fun unitSteps(n: Int) {
  if (n == 0) return
  steps++
  unitSteps(n - 1)
  return
}

suspend fun maybe(n: Int): Int? = if (n == 0) 7 else null

tailrec suspend fun elvis(n: Int): Int = maybe(n) ?: elvis(n - 1)

object Walker {
  tailrec suspend fun walk(n: Int, acc: Int = 0): Int = if (n == 0) acc else Walker.walk(n - 1, acc + 1)
}

// Suspends before each tail call, with a local and a parameter live
// across the suspension.
tailrec suspend fun countDown(n: Int, acc: Long): Long {
  if (n == 0) return acc
  val twice = n.toLong() * 2
  pause()
  return countDown(n - 1, acc + twice)
}

// Suspends in the tail call's arguments.
tailrec suspend fun sumGot(n: Int, acc: Long = 0): Long = if (n == 0) acc else sumGot(n - 1, acc + get(n))

// Local suspend tailrec functions: in tail form, and suspending before
// each tail call, with a captured var.
suspend fun localSum(n: Int): Long {
  tailrec suspend fun go(i: Int, acc: Long = 0): Long = if (i == 0) acc else go(i - 1, acc + i)
  return go(n)
}

suspend fun localPauses(n: Int): Int {
  var seen = 0
  tailrec suspend fun go(i: Int, step: Int = 1): Int {
    if (i <= 0) return seen
    pause()
    seen++
    return go(i - step)
  }
  return go(n, 2)
}

fun main() {
  println(drive { sum(1_000_000) })
  println(drive { 1_000_000 down 0 })
  println(drive { count("x".repeat(1_000_000), 0) })
  println(drive { (1..1_000_000).iterator().foldl(0L) { a, x -> a + x } })
  drive { unitSteps(1_000_000) }
  println(steps)
  println(drive { elvis(1_000_000) })
  println(drive { Walker.walk(1_000_000) })
  println(drive { countDown(100_000, 0) })
  println(pauses)
  println(drive { sumGot(100_000) })
  println(pauses)
  println(drive { localSum(1_000_000) })
  println(drive { localPauses(100_000) })
  println(pauses)
}

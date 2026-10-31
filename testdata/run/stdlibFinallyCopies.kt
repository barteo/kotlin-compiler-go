// Non-local jumps out of lambdas that library inline functions invoke in
// a try: the copy runs the function's finally block first, which for
// ReentrantReadWriteLock.write is two loops, relocking the read holds.
import java.util.concurrent.locks.ReentrantLock
import java.util.concurrent.locks.ReentrantReadWriteLock
import kotlin.concurrent.read
import kotlin.concurrent.withLock
import kotlin.concurrent.write

val rw = ReentrantReadWriteLock()
val lock = ReentrantLock()
val cache = HashMap<String, Int>()

fun state() = "write=${rw.isWriteLocked} reads=${rw.readLockCount} lock=${lock.isLocked}"

fun get(key: String): Int {
    rw.read { cache[key]?.let { return it } }
    rw.write {
        cache[key]?.let { return it }
        return key.length.also { cache[key] = it }
    }
}

fun upgrade(key: String): Int {
    rw.read {
        rw.write {
            if (key.isEmpty()) return -rw.readHoldCount
            cache[key] = 0
        }
        return rw.readHoldCount
    }
}

fun sum(xs: List<Int>): Int {
    var total = 0
    for (x in xs) {
        rw.write {
            if (x < 0) break
            if (x == 0) continue
            lock.withLock {
                if (x > 100) return -1
                total += x
            }
        }
    }
    return total
}

fun main() {
    println(get("abc"))
    println(state())
    println(get("abc"))
    println(state())
    println(upgrade(""))
    println(state())
    println(upgrade("k"))
    println(state())
    println(sum(listOf(1, 0, 2, -1, 5)))
    println(state())
    println(sum(listOf(1, 200)))
    println(state())
}

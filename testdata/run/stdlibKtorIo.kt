// The constructs that ktor-io needed.

import kotlin.coroutines.*
import kotlin.random.Random

// A property that can't be invoked doesn't hide an extension function of
// its name: ktor's ByteReadChannel has a readBuffer property and a
// readBuffer() extension.
interface Channel {
    val readBuffer: String
}
fun Channel.readBuffer(): Int = readBuffer.length
fun Channel.readBuffer(max: Int): Int = minOf(readBuffer(), max)
fun Channel.total(): Int = readBuffer() + readBuffer(2)

// An explicit import's functions and constructors that can't take the
// arguments give way to the package's: ktor's ByteString(ByteBuffer)
// next to an import of kotlinx.io's ByteString.
fun Random(name: String): Random = Random(name.length)
fun pick(name: String) = Random(name).nextInt(100) == Random(name.length).nextInt(100)

// A type parameter's bound that the receiver doesn't fit makes an
// extension give way: ktor's T.use, for T : Closeable?, to the standard
// library's for an AutoCloseable.
inline fun <T : java.io.Closeable?, R> T.use(block: (T) -> R): R =
    try {
        block(this)
    } finally {
        this?.close()
        println("closed by the package's use")
    }
class Resource(val name: String) : AutoCloseable {
    override fun close() = println("closed $name")
}
fun read(r: Resource): String = r.use { it.name.uppercase() }
fun reader(): Int = java.io.StringReader("k").use { it.read() }

// An integer literal branch takes the type of the others: 0 is a Long,
// as in ktor's scanForSoleCr.
fun index(lf: Long, limit: Long): Long {
    val end = when (lf) {
        -1L -> minOf(limit, 3L)
        0L -> 0
        else -> lf - 1
    }
    return takesLong(end)
}
fun takesLong(n: Long) = n * 2
fun short(b: Boolean, x: Int): Short {
    val s = if (b) x.toShort() else -1
    return s
}
fun maybe(n: Int): Long? {
    val v = when (n) {
        0 -> null
        1 -> 5L
        else -> 8 * 1024
    }
    return v
}
fun mixed(b: Boolean): Any = if (b) 1.5 else 0

// A typealias of Any is constructed as Any: ktor's actual typealias
// SynchronizedObject = Any.
typealias Lock = Any
class Guarded {
    private val lock = Lock()
    fun next(n: Int): Int = synchronized(lock) { n + 1 }
}

// The standard library's inline-only companion members, as ktor's
// Result.success(Unit) and Result.failure(cause).
class Task(val continuation: (Result<Unit>) -> Unit) {
    fun resume(cause: Throwable? = null) =
        continuation(cause?.let { Result.failure(it) } ?: RESUME)

    companion object {
        val RESUME = Result.success(Unit)
    }
}

// Unit?, a kotlin.Unit or null: ktor's throwOrNull.
inline fun throwOrNull(cause: Throwable?, wrap: (Throwable) -> Throwable): Unit? =
    cause?.let { throw wrap(it) }
fun unitOrNull(b: Boolean): Unit? = if (b) Unit else null

// A when on a property of the implicit extension receiver smart casts it:
// ktor's CloseToken.wrapCause.
class Token(private val origin: Throwable?) {
    companion object {
        fun Token.cause(wrap: (Throwable) -> Throwable): Throwable? {
            return when (origin) {
                null -> null
                is IllegalStateException -> IllegalStateException(origin.message, origin)
                is UnsupportedOperationException -> origin
                else -> wrap(origin)
            }
        }
    }
}

// A local var that only a local function assigns stays smart cast in the
// function, and around it where each value it assigns has the narrowed
// type: ktor's internalReadLineTo.
fun lines(xs: List<String>): String {
    var out: StringBuilder? = null
    var consumed = 0
    fun buffer(s: String) {
        if (out == null) {
            out = StringBuilder()
        }
        out.append(s)
        consumed += s.length
    }
    try {
        xs.forEach { buffer(it) }
    } finally {
        if (out != null) println(out.length)
    }
    return "$out $consumed"
}

// A nested class's constructor through its outer type, for a reified type
// parameter: ktor's sleepWhile(Slot::Read).
sealed interface Slot {
    sealed interface Task : Slot { val n: Int }
    class Read(override val n: Int) : Task
    class Write(override val n: Int) : Task
}
inline fun <reified T : Slot.Task> make(create: (Int) -> T): String {
    val t = create(3)
    return "${T::class.simpleName} ${t.n}"
}

// A reference to a suspend member of the implicit receiver, a suspend
// function value: ktor's ::flushAndClose.fireAndForget().
interface Chan { suspend fun flushAndClose() }
fun <R> (suspend () -> R).fireAndForget() {
    startCoroutine(Continuation(EmptyCoroutineContext) { println("done $it") })
}
fun Chan.close() { ::flushAndClose.fireAndForget() }
suspend fun top() = println("top")

// A private const's value is inlined where another class reads it, also
// when it divides or converts: ktor's DefaultPool and MAX_CAPACITY.
private const val MULTIPLIER = 4
private const val MAGIC = 2654435769.toInt()
private const val MAX_CAPACITY = Int.MAX_VALUE / MULTIPLIER
private const val REST = -7 % 3
private const val NARROW = 0x1234.toByte()
class Pool(capacity: Int) {
    init {
        require(capacity <= MAX_CAPACITY) { "capacity at most $MAX_CAPACITY" }
    }
    fun hash(n: Int) = (n * MAGIC) ushr 30
    override fun toString() = "$MAX_CAPACITY $MAGIC $REST $NARROW"
}

// A @JvmStatic member of an interface's companion is a static method of
// the interface, which can't be final: ktor's ByteChannel.Slot.CLOSED.
sealed interface Slot2 {
    companion object {
        @JvmStatic
        val CLOSED = Closed(null)

        @JvmStatic
        fun of(cause: Throwable?): Slot2 = if (cause == null) CLOSED else Closed(cause)
    }
    class Closed(val cause: Throwable?) : Slot2
}

fun main() {
    println(object : Channel {
        override val readBuffer = "abcd"
    }.total())
    println(pick("seed"))
    println(read(Resource("r")))
    println(reader())
    println(index(-1L, 10L))
    println(index(0L, 10L))
    println(index(7L, 10L))
    println(short(false, 3))
    println(maybe(0))
    println(maybe(2))
    println(mixed(false)::class.simpleName)
    println(Guarded().next(41))
    Task { println(it) }.resume()
    Task { println(it.exceptionOrNull()?.message) }.resume(IllegalStateException("boom"))
    println(throwOrNull(null) { it })
    println(runCatching { throwOrNull(Exception("a")) { IllegalStateException("wrapped " + it.message) } }.exceptionOrNull()?.message)
    println(unitOrNull(true))
    println(unitOrNull(false))
    with(Token.Companion) {
        println(Token(null).cause { it })
        println(Token(IllegalStateException("a")).cause { it }?.message)
        println(Token(RuntimeException("b")).cause { Exception("w " + it.message) }?.message)
    }
    println(lines(listOf("ab", "c")))
    println(make(Slot::Read))
    println(make(Slot::Write))
    val ctor = Slot::Read
    println(ctor(4).n)
    object : Chan { override suspend fun flushAndClose() = println("flushed") }.close()
    ::top.fireAndForget()
    println(Pool(8))
    println(Pool(8).hash(3))
    println(runCatching { Pool(Int.MAX_VALUE) }.exceptionOrNull()?.message)
    println(Slot2.CLOSED.cause)
    println((Slot2.of(Exception("e")) as Slot2.Closed).cause?.message)
}

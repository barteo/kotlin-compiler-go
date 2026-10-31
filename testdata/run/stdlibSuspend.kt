import kotlin.coroutines.*
import kotlin.coroutines.intrinsics.*
import kotlin.coroutines.jvm.internal.CoroutineStackFrame

// Suspends for real: another thread resumes the coroutine with v.
suspend fun <T> later(v: T): T = suspendCoroutineUninterceptedOrReturn { c ->
    Thread { c.resume(v) }.start()
    COROUTINE_SUSPENDED
}

// Suspends, then resumes the coroutine with an exception.
suspend fun fail(msg: String): Int = suspendCoroutineUninterceptedOrReturn { c ->
    Thread { c.resumeWithException(IllegalStateException(msg)) }.start()
    COROUTINE_SUSPENDED
}

// Doesn't suspend: returns the value at once.
suspend fun now(v: Int): Int = suspendCoroutineUninterceptedOrReturn { v }

suspend fun kinds(): String {
    val b: Byte = later(1.toByte())
    val s: Short = later(2.toShort())
    val i = later(3)
    val l = later(4L)
    val f = later(5.5f)
    val d = later(6.25)
    val c = later('x')
    val z = later(true)
    val str = later("str")
    val arr = later(intArrayOf(7, 8))
    val n: String? = later(null)
    return "$b $s $i $l $f $d $c $z $str ${arr[1]} $n"
}

suspend fun loops(n: Int): Int {
    var sum = 0
    for (i in 0 until n) {
        sum += later(i)
    }
    var k = 0
    while (k < n) {
        k = later(k + 1)
    }
    for (x in listOf(10, 20)) {
        sum += later(x) * 2
    }
    return sum + k
}

suspend fun branches(x: Int): String {
    val w = when (later(x)) {
        1 -> "one"
        2 -> later("two")
        else -> "many"
    }
    val e = later<String?>(null) ?: later("default")
    val t = if (x > 1) later("big") else "small"
    return "$w $e $t ${later(x * 10)}"
}

// Values on the operand stack across suspension points.
suspend fun stack(a: Int): Long {
    val x = a + later(1) * (later(2) + a)
    val y = 10L - later(3L)
    val s = "a" + later("b") + x
    return x + y + s.length
}

class Point(val x: Int, val y: String)

suspend fun construct(): String {
    val p = Point(later(1), later("y"))
    val q = Point(x = now(2), y = "z" + later("w"))
    return "${p.x}${p.y}${q.x}${q.y}"
}

suspend fun tries(): String {
    val sb = StringBuilder()
    try {
        sb.append(later("a"))
        sb.append(fail("boom"))
        sb.append("not reached")
    } catch (e: IllegalStateException) {
        sb.append(" caught ").append(e.message)
        sb.append(later(" after"))
    } finally {
        sb.append(" finally")
        sb.append(later("!"))
    }
    val v = try {
        fail("again")
    } catch (e: Exception) {
        later(42)
    }
    sb.append(" ").append(v)
    return sb.toString()
}

suspend fun rethrows(): Int {
    try {
        return fail("out")
    } finally {
        println("finally runs")
    }
}

// Tail calls: no continuation class.
suspend fun tailValue(x: Int): Int = later(x + 1)

suspend fun tailIf(b: Boolean): Int = if (b) later(1) else now(2)

suspend fun tailReturn(x: Int): Int {
    if (x > 5) return later(x)
    return tailValue(x)
}

suspend fun tailUnit(msg: String) {
    say(msg)
}

suspend fun say(msg: String) {
    later(Unit)
    println(msg)
}

suspend fun <T> generic(v: T): List<T> = listOf(later(v), v)

suspend fun nullable(s: String?): Int? {
    val n = later(s)?.length
    return n
}

suspend fun recursion(n: Int): Int = if (n == 0) 0 else n + recursion(later(n - 1))

suspend fun locals(n: Int): Int {
    suspend fun twice(x: Int): Int = later(x) + later(x)
    var acc = 0
    for (i in 1..n) {
        acc += twice(i)
    }
    return acc
}

suspend fun context(): String = coroutineContext.toString()

// The coroutine's stack, as debuggers see it: each frame from its
// continuation's @DebugMetadata.
suspend fun frames(): List<String> = suspendCoroutineUninterceptedOrReturn { c ->
    val out = ArrayList<String>()
    var f: CoroutineStackFrame? = c as? CoroutineStackFrame
    while (f != null) {
        val e = f.getStackTraceElement()
        if (e != null) {
            out.add(e.className + "." + e.methodName + " " + e.fileName + ":" + e.lineNumber)
        }
        f = f.callerFrame
    }
    out
}

suspend fun outer(n: Int): String {
    val s = middle(n)
    return s
}

suspend fun middle(n: Int): String {
    later(n)
    val fs = frames()
    return fs.joinToString("\n")
}

suspend fun main() {
    println(kinds())
    println(loops(4))
    println(branches(1))
    println(branches(2))
    println(branches(3))
    println(stack(5))
    println(construct())
    println(tries())
    try {
        rethrows()
    } catch (e: IllegalStateException) {
        println("rethrown " + e.message)
    }
    println(tailValue(1))
    println(tailIf(true) + tailIf(false))
    println(tailReturn(7) + tailReturn(1))
    tailUnit("unit")
    println(generic("g"))
    println(nullable("four"))
    println(nullable(null))
    println(recursion(5))
    println(locals(3))
    println(context())
    println(outer(1))
}

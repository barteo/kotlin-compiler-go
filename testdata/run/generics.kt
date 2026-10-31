// Generic classes and functions declared in Kotlin, with inference.

class Box<T>(var value: T) {
    fun get(): T = value
    fun set(v: T) {
        value = v
    }
    fun copy() = Box(value)
    fun describe(): String = "Box(" + value + ")"
}

class Pair<A, B>(val first: A, val second: B) {
    fun swap(): Pair<B, A> = Pair(second, first)
}

class NonNull<T : Any>(val v: T) {
    fun orNull(flag: Boolean): T? = if (flag) v else null
}

fun <T> id(x: T): T = x

fun <T> twice(x: T) = Pair(x, x)

fun <T> firstOf(p: Pair<T, T>): T = p.first

fun <T> pick(flag: Boolean, a: T, b: T): T = if (flag) a else b

fun <T> isNull(x: T): Boolean = x == null

fun <T> same(a: T, b: T): Boolean = a == b

fun <T> show(x: T): String = "[$x]"

fun <T : Any> orDefault(x: T?, d: T): T = x ?: d

fun <T> unbox(b: Box<T>): T = b.value

fun takesLongBox(b: Box<Long>): Long = b.value + 1

fun <T> count(x: T, n: Int): Int = if (n == 0) 0 else 1 + count(x, n - 1)

fun main() {
    // Inference from constructor arguments.
    val b = Box(1)
    println(b.value + 1)
    b.value = 41
    b.value += 1
    b.value++
    println(b.get())
    b.set(7)
    println(b.describe())
    val s = Box("hello")
    println(s.value.length)
    println(s.copy().get().uppercase())

    // Explicit type arguments.
    val n = Box<Int?>(null)
    println(n.value ?: 0)
    n.value = 5
    println(n.value)
    println(id<String>("explicit"))

    // Generic functions.
    println(id(3) * 2)
    println(id("x") + id("y"))
    val p = Pair(1, "one")
    println(p.first + 1)
    println(p.second.length)
    val q = p.swap()
    println(q.first + q.second)
    println(firstOf(twice(5)) + 1)
    println(pick(true, "a", "b"))
    println(pick(false, 1, 2))
    println(isNull(null))
    println(isNull("x"))
    println(same(1, 1))
    println(same("a", "b"))
    println(show(1) + show(null) + show("z"))
    println(orDefault(null, "default"))
    println(orDefault("given", "default"))
    println(unbox(Box(2.5)) * 2)
    println(count("x", 3))

    // The expected type decides Long for integer literals.
    val l: Long = id(1)
    println(l)
    val lb: Box<Long> = Box(2)
    println(lb.value * 3)
    println(takesLongBox(Box(10)))

    // Nested calls are inferred together.
    println(Box(Box(4)).value.value + 1)
    println(id(id(9L)))
    println(unbox(Box(Box("deep"))).value)
    val nb: Box<String?> = Box(null)
    println(nb.value)

    // Nullable results and type parameters bounded by Any.
    val nn = NonNull("v")
    println(nn.orNull(true))
    println(nn.orNull(false))
    println(nn.orNull(true)?.length)
}

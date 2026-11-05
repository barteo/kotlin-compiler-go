// Calls through projected types, whose arguments are captured.
class Box<T>(var v: T) {
    fun get(): T = v
    fun each(f: (T) -> Unit) { f(v) }
}

fun <T> first(b: Box<T>): T = b.get()
fun <T> Box<T>.ext(f: (T) -> Unit) { f(get()) }
fun <T> Box<T>.pair(f: (T, T) -> Unit, g: (T, T) -> Unit) { f(get(), get()); g(get(), get()) }
fun <T> copy(b: Box<T>): Box<T> = Box(b.get())

fun outBox(b: Box<out Number>) {
    println(first(b).toInt() + 1)
    b.each { println(it.toDouble()) }
    b.ext { println(it.toLong()) }
    b.pair({ x, y -> println(x.toInt() + y.toInt()) }) { u, w -> println(u.toInt() * w.toInt()) }
    val c: Box<out Number> = copy(b)
    println(c.get())
}

fun inBox(b: Box<in Int>) {
    val a: Any? = first(b)
    println(a)
    b.each { println("in: $it") }
}

fun stars(xs: Array<Box<*>>) {
    for (x in xs) println(first(x))
}

fun main() {
    outBox(Box(3))
    inBox(Box<Number>(4))
    stars(arrayOf(Box("s"), Box(5)))
    val arr: Array<out Box<Int>> = arrayOf(Box(6))
    println(first(arr[0]) + 1)
}

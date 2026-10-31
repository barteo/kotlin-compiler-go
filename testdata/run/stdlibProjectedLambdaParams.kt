import java.util.concurrent.CompletableFuture
import java.util.function.Consumer

fun describe(f: CompletableFuture<*>): CompletableFuture<String> =
    f.handle { _, e -> e?.let { it.message ?: "no message" } ?: "ok" }

fun describeNamed(f: CompletableFuture<*>): CompletableFuture<String> =
    f.handle { v, e -> if (e == null) "value $v" else "failed: ${e.message}" }

class Box<T>(val t: T) {
    fun use(f: (T) -> String) = f(t)
    fun useJava(c: Consumer<in T>) = c.accept(t)
}

fun show(b: Box<*>) {
    println(b.use { v -> if (v is Int) "int $v" else "other $v" })
    b.useJava { v -> println("java ${v?.hashCode() ?: 0} $v") }
}

fun lengths(fs: MutableList<Consumer<out CharSequence>>) {
    fs.add { v -> println("length ${v.length}") }
}

val pick: (Nothing?, Int) -> Int = { n, x -> if (n == null && x > 0) x else -x }

fun main() {
    println(describe(CompletableFuture.completedFuture(1)).get())
    val failed = CompletableFuture<Int>()
    failed.completeExceptionally(IllegalStateException("boom"))
    println(describe(failed).get())
    println(describeNamed(CompletableFuture.completedFuture("x")).get())
    println(describeNamed(failed).get())
    show(Box(4))
    show(Box("s"))
    val fs = ArrayList<Consumer<out CharSequence>>()
    lengths(fs)
    @Suppress("UNCHECKED_CAST")
    (fs[0] as Consumer<String>).accept("four")
    println(pick(null, 3))
    println(pick(null, -4))
}

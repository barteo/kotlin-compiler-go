import api.*
import java.lang.reflect.Modifier

fun methods(c: Class<*>) = c.declaredMethods.map { it.name }.filter { !it.startsWith("access$") }.sorted()

class Fixed : Cache(10) {
    override val size: Int get() = 42
}

fun main() {
    println(describe(Result.success(2)))
    println(describe(Result.failure(Exception("no"))))
    println(Result.success(4).twice())
    println(Holder(Result.success(5)).r.getOrNull())

    val d = Date(2024, 5)
    println(d.copy(month = 6))
    println(d.copy())

    println(Mode.of(Mode.A, Mode.B).has(Mode.A))
    val impl = Mode::class.java.getDeclaredMethod("constructor-impl", Int::class.javaPrimitiveType)
    println(Modifier.toString(impl.modifiers))

    val c = Cache(2)
    c["a"] = 1
    c["b"] = 2
    c["c"] = 3
    println(c.keys)
    println(c.size)
    val m: Map<String, Int> = Fixed()
    println(m.size)
    val n = Names()
    n.add("x")
    n.add("y")
    n.removeAt(0)
    println(n)

    println(Chars(charArrayOf('h', 'i'))[1])
    println(methods(Tag::class.java))
    println(methods(Class.forName("api.Point\$DefaultImpls")))
    println(runCatching { Class.forName("api.Narrow\$DefaultImpls") }.isSuccess)

    println(Class.forName("api.Parts").getField("LIMIT").getInt(null))
    println(methods(Class.forName("api.Parts__PartKt")))
    println(limited(40))

    println(source.value)
    println(methods(source.javaClass))
    println(methods(Class.forName("api.ApiKt")))
}

import java.util.concurrent.atomic.AtomicReference

class Info {
    var frequency = 0
}

// A value read from a Java class through a star or in projection has a
// platform type, Any!, which a non-null type takes.
fun make(cls: Class<*>, frequency: Int): Any =
    cls.getDeclaredConstructor().newInstance().also { info ->
        cls.getMethod("setFrequency", Int::class.javaPrimitiveType).invoke(info, frequency)
    }

fun read(ref: AtomicReference<*>): Any = ref.get()

fun readIn(ref: AtomicReference<in String>): Any = ref.get()

fun readNullable(ref: AtomicReference<*>): Any? = ref.get()

fun main() {
    println((make(Info::class.java, 5) as Info).frequency)
    println(read(AtomicReference("a")))
    println(readIn(AtomicReference<CharSequence>("b")))
    println(readNullable(AtomicReference<String>(null)))
    val text = read(AtomicReference("c")).toString()
    println(text.length)
    try {
        read(AtomicReference<String>(null))
        println("no check")
    } catch (e: NullPointerException) {
        println("checked")
    }
}

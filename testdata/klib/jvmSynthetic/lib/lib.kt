package lib

interface Ops {
    @JvmSynthetic
    infix fun String.eq(o: Int): String

    @JvmSynthetic
    fun count(): Int
}

class OpsImpl : Ops {
    override fun String.eq(o: Int) = "$this = $o"
    override fun count() = 7
}

fun <T> withOps(f: Ops.() -> T): T = OpsImpl().f()

@JvmSynthetic
fun plusOne(x: Int) = x + 1

@get:JvmSynthetic
val answer: Int get() = 42

class Box(@set:JvmSynthetic var value: String) {
    @JvmSynthetic
    fun shout() = value.uppercase()
}

inline fun <reified T> typeName(): String = T::class.java.simpleName

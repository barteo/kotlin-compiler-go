// A value class's override of an interface's generic function with a
// primitive result returns the box, as kotlinc's does, and so do its
// bridges' calls of it: IntFactory's invoke(): Int is invoke() of
// Integer, which its bridge invoke() of Object calls.

fun interface Factory<T> {
    fun invoke(): T
}

interface Mapper<T> {
    fun map(input: T): T
}

@JvmInline
value class IntFactory(val value: Int) : Factory<Int> {
    override fun invoke(): Int = value
}

@JvmInline
value class BooleanFactory(override val value: Boolean) : Factory<Boolean>, Lazy<Boolean> {
    override fun isInitialized(): Boolean = true

    override fun invoke(): Boolean = value
}

@JvmInline
value class Offset(val by: Long) : Mapper<Long> {
    override fun map(input: Long): Long = input + by
}

fun <T> produce(factory: Factory<T>): T = factory.invoke()

fun main() {
    val ints: Factory<Int> = IntFactory(3)
    val booleans: Factory<Boolean> = BooleanFactory(true)
    val lazy: Lazy<Boolean> = BooleanFactory(false)
    val mapper: Mapper<Long> = Offset(5)
    println("${ints.invoke()} ${booleans.invoke()} ${lazy.value} ${mapper.map(10)}")
    println("${produce(IntFactory(4))} ${IntFactory(5).invoke() + 1} ${Offset(1).map(2)}")
}

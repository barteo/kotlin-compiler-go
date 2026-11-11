package b

abstract class Check<out T>(val name: String?) {
    abstract fun <R> checkThat(v: R, name: String? = this.name): Check<R>

    abstract val value: T

    inline fun <R> transform(name: String? = this.name, transform: (T) -> R): Check<R> =
        try {
            checkThat(transform(value), name)
        } catch (e: Throwable) {
            println("failed: ${e.message}")
            this as Check<R>
        }
}

class Value<out T>(override val value: T, name: String?) : Check<T>(name) {
    override fun <R> checkThat(v: R, name: String?): Check<R> = Value(v, name)

    override fun toString() = "$name=$value"
}

fun <T> checkThat(v: T, name: String? = null): Check<T> = Value(v, name)

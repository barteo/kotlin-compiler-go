import kotlin.reflect.KClass

annotation class AnnArray(val a: Array<KClass<*>>)

@AnnArray(arrayOf(String::class, Array<String>::class, Array<Array<Int>>::class))
fun ok() {}

inline val <reified T> T.test
    get() = @AnnArray(arrayOf( // ERROR only 'const val' can be used in constant expressions
        T::class, // ERROR annotation argument must be a compile-time constant
        Array<T>::class, // ERROR annotation argument must be a compile-time constant
        Array<Array<T>>::class // ERROR annotation argument must be a compile-time constant
    )) object {}

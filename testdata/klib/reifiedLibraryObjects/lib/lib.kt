package reif

// kotlinc compiles these inline functions, whose bodies inline the
// standard library's filterIsInstance, which creates an object that
// operates on its own reified type parameter: the copy in each function's
// method must stay reifiable, so that a program copying the method copies
// the object with its own type arguments, as kotlinc's inliner does.

inline fun <reified T> Sequence<*>.only(): List<T> = filterIsInstance<T>().toList()

inline fun <reified T> Iterable<*>.firstOf(): T? = asSequence().filterIsInstance<T>().firstOrNull()

inline fun <reified T> Sequence<*>.orNulls(): List<T?> = filterIsInstance<T?>().toList()

// A chain: copies of copies.
inline fun <reified E> Sequence<*>.count2(): Int = only<E>().size

// One type argument kept, one concrete.
inline fun <reified A> Sequence<*>.andStrings(): Pair<List<A>, List<String>> =
    filterIsInstance<A>().toList() to filterIsInstance<String>().toList()

interface Picker {
    fun pick(xs: Sequence<Any?>): List<Any?>
}

// An object of the function that inlines filterIsInstance in its method.
inline fun <reified T> picker(): Picker = object : Picker {
    override fun pick(xs: Sequence<Any?>): List<Any?> = xs.filterIsInstance<T>().toList()
}

// Casts to the type arguments: a non-null one throws for null.
inline fun <reified T> cast(x: Any?): T = x as T

inline fun <reified T> List<Any?>.castAll(): List<T> = map { it as T }

// An object of the standard library's map that casts, and a copy of it.
inline fun <reified T> List<Any?>.castSeq(): List<T> = asSequence().map { it as T }.toList()

inline fun <reified E> List<Any?>.castSeq2(): List<E> = castSeq<E>()

package fixture

// Calls, null safety and casts: rules that resolve calls and types.

fun length(s: String): Int = s?.length ?: 0

fun forced(s: String): Int = s!!.length

fun mapLookup(m: Map<String, Int>): Int = m["a"]!!

fun cast(x: Any): String = x as String

fun nullableCast(x: Any?): String? = x as String?

fun nullableString(s: String?): String = s.toString()

fun chars(c: CharArray): String = c.toString()

fun orEmpty(s: String?): String = s ?: ""

fun isNullOrEmpty(s: String?): Boolean = s == null || s.isEmpty()

fun uselessCall(s: String): String = s.orEmpty()

fun ignored(list: List<Int>) {
    list.map { it + 1 }
    listOf(1, 2).filter { it > 1 }
}

fun letUnneeded(s: String): Int = s.let { it.length }

fun stringCompare(a: String, b: String): Boolean = a === b

@Deprecated("old")
fun old(): Int = 1

fun usesOld(): Int = old()

fun unreachable(): Int {
    return 1
    println("never")
}

fun elvisUseless(s: String): String = s ?: "x"

fun namedArgs(a: Int, b: Int, c: Int, d: Int): Int = a + b + c + d

fun callsNamed(): Int = namedArgs(1, 2, 3, 4)

fun implicitUnit() = println("unit")

fun exitHere() {
    System.exit(1)
}

class Spec(val modifiers: Set<String>, val body: List<Int>?)

fun safeChain(s: Spec?): Boolean = s?.modifiers?.contains("inline") ?: false

fun safeChainMember(s: Spec?): Boolean = s?.body?.isEmpty() ?: true

// A function type's invoke has no stable parameter names, called as
// f(a, b) or f.invoke(a, b): no UnnamedParameterUse.
fun invokes(f: ((a: Any?, b: Any?) -> Unit)?, x: Any?, y: Any?) {
    f?.invoke(x, y)
}

// get of a ClassValue<String> object is a substitution override, not
// ClassValue's Java method: ExplicitCollectionElementAccessMethod.
private class Cache {
    private val values = object : ClassValue<String>() {
        override fun computeValue(type: Class<*>?): String = type.toString()
    }

    fun of(type: Class<*>): String = values.get(type)
}

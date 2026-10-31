// A type variable inferred from a Java value's platform type keeps it, as
// in Kotlin: T = String! then also takes null, as mockk's
// every { prefs.getString(key, null) } returns null.
class Scope<T>(val value: T)

fun <T> every(block: () -> T): Scope<T> = Scope(block())

infix fun <T> Scope<T>.returns(v: T): String = "returns $v instead of $value"

fun <T> pick(a: T, b: T) = listOf(a, b)

fun stamp(last: Long?) = every { java.lang.Long.getLong("kc.none") } returns last

fun main() {
    println(every { System.getProperty("kc.none") } returns null)
    println(every { System.getProperty("java.specification.name") } returns "x")
    println(stamp(null))
    println(stamp(4L))
    println(every { Integer.getInteger("kc.none") } returns null)
    val names = mutableListOf(System.getProperty("kc.none"))
    names.add(null)
    names.add("b")
    println(names)
    println(pick(System.getProperty("kc.none"), null))
    val n: Long = 5
    println(every { n } returns 6)
}

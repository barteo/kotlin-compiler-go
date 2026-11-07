import j.Asserts.assertThat
import j.Box
import j.boxOf

fun main() {
    val b = Box<Boolean>(true)
    println(assertThat(b.getValue() as Boolean))
    println(assertThat(b.value as Boolean))
    println(assertThat(boxOf(false).value as Boolean))
    val any: Any = false
    println(assertThat(any as Boolean))
    println(assertThat(b.value as Boolean?))
    val n = Box<Boolean?>(null)
    println(runCatching { assertThat(n.value as Boolean) }.exceptionOrNull()?.javaClass?.simpleName)
}

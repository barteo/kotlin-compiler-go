import j.A
import j.EditText
import j.Maybe
import j.editable

fun main() {
    println(Maybe(true).isEmpty.blockingGet())
    val r = runCatching { Maybe(false).isEmpty.blockingGet() }
    println(r.getOrNull())
    val e = EditText()
    e.text = editable("abc")
    println(e.text.length)
    e.text = null
    println("[" + e.text + "]")
    val a = A()
    println(a.isFoo)
    println(a.isBaz)
    a.isBaz = 3
    println(a.isBoxed)
}

import view.View
import view.textColor

// The extension's setter, not the member, both on an implicit receiver
// and on an explicit one.
fun main() {
    val v = View()
    v.apply { textColor = 5 }
    println(v.color)
    v.textColor = 6
    println(v.color)
}

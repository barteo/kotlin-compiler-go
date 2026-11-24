class View(val turn: Int)

class Holder(var view: View?)

fun tap(slot: Int, turn: Int) = println("tap $slot $turn")

fun main() {
    val holder = Holder(View(3))
    val before = holder.view
    tap(0, requireNotNull(before).turn)
    tap(1, before.turn)

    val other = holder.view
    val turn = checkNotNull(other).turn + 1
    println(turn + other.turn)

    val third = holder.view
    var sum = 0
    sum = sum + requireNotNull(third) { "no view" }.turn
    println(sum + third.turn)

    val text: String? = if (sum > 0) "text" else null
    println("${requireNotNull(text).length}")
    println(text.length)

    val skipped: View? = null
    println(skipped?.let { tap(2, requireNotNull(holder.view).turn) })
    println(holder.view?.turn)
}

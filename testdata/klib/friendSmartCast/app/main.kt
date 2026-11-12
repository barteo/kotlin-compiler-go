// friend
import lib.*

fun check(result: Result) {
    println(result.inspection!!.id)
    println(result.inspection.end)
    if (result.inspection.end != null) {
        println(result.inspection.end.length)
    }
}

fun main() {
    check(Result(1, Inspection(2, "end")))
}

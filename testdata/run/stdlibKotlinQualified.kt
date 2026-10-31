// Calls of the kotlin package's array functions by their qualified names,
// in code and in annotation arguments.

annotation class Keys(val names: Array<String>)

@Keys(names = kotlin.arrayOf("id"))
class Record

fun test(y: Array<in Array<String>>) {
    y[0] = kotlin.arrayOf("OK")
}

fun main() {
    val x: Array<Array<*>> = kotlin.arrayOf(kotlin.arrayOf(1))
    test(x)
    println(x[0][0])
    println(kotlin.intArrayOf(1, 2).sum())
    println(Record::class.java.getAnnotation(Keys::class.java) == null)
}

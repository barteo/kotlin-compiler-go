// Lambdas converted to fun interfaces and Java interfaces outside call
// arguments (ResolveTopLevelLambdasAsSyntheticCallArgument, on since
// Kotlin 2.3): initializers, returns, assignments and when branches.
fun interface MyFun {
    fun foo(x: String): Int
}

fun interface Suspending {
    suspend fun run(): String
}

val topLevel: MyFun = { it.length }
val nullable: MyFun? = { it.length * 10 }

fun returned(): MyFun = { it.length + 1 }

fun explicitReturn(): Runnable {
    return { println("runnable") }
}

class Holder(val f: MyFun = { -it.length })

lateinit var late: MyFun

fun main() {
    println(topLevel.foo("abc"))
    println(nullable?.foo("abc"))
    println(returned().foo("abc"))
    explicitReturn().run()
    println(Holder().f.foo("abc"))
    var local: MyFun = { it.length }
    local = { it.length * 2 }
    println(local.foo("abc"))
    late = { 42 }
    println(late.foo(""))
    val w: MyFun = when {
        "x".length == 1 -> { { 7 } }
        else -> { { 8 } }
    }
    println(w.foo(""))
    val cmp: Comparator<String> = { a, b -> a.length - b.length }
    println(listOf("ccc", "a", "bb").sortedWith(cmp))
    val s: Suspending = { "suspend" }
    println(s is Suspending)
}

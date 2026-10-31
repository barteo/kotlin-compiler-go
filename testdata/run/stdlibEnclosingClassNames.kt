// Classes made in accessors are named after the property, in default
// values after the function, and in a constructor's after the class, as
// kotlinc names them.

class Tree {
    val children: Sequence<Int> get() = sequence { yield(1) }
    val block: suspend () -> Int get() = { 1 }
    var runner: Runnable
        get() = object : Runnable { override fun run() {} }
        set(v) {
            val wrapped = object : Runnable { override fun run() = v.run() }
            last = wrapped
        }
    var last: Runnable? = null
}

val Int.digits: Sequence<Int> get() = sequence { yield(this@digits % 10) }
val Int.task: suspend () -> Int get() = { this@task + 1 }

fun retry(n: Int = 1, predicate: suspend (Int) -> Boolean = { it > n }) = predicate

class Job(val step: suspend () -> Int = { 1 })

fun task(r: Runnable = object : Runnable { override fun run() {} }) = r

fun main() {
    val t = Tree()
    println(t.children.toList())
    println(t.block.javaClass.name)
    println(t.runner.javaClass.name)
    t.runner = Runnable {}
    println(t.last!!.javaClass.name)
    println(42.digits.toList())
    println(42.task.javaClass.name)
    println(retry().javaClass.name)
    println(task().javaClass.name)
    println(Job().step.javaClass.name)
}

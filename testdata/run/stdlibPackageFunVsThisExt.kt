// A function of the file's package that isn't an extension gives way to
// the standard library's extension of an implicit receiver, T.run, as in
// K2: run { } in a class doesn't call the package's run(block), though
// it does outside one.

fun run(block: () -> Unit) {
    println("package run")
}

class C {
    val p = run { println("block p") }

    fun f() {
        run { println("block f") }
        val r = run { 42 }
        println(r)
    }
}

fun main() {
    C().f()
    run { println("block main") }
}

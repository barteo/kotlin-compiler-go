fun apply(x: Int, f: (Int) -> Int): Int = f(x)
fun <T> run(f: () -> T): T = f()
fun pair(f: (Int, Int) -> Int) = f(1, 2)

class C(val n: Int) {
    fun m(): () -> Int = { n }
}

fun main() {
    val a = { it * 2 } // ERROR unresolved reference: it
    val b = { x -> x } // ERROR cannot infer a type for this parameter
    val c: (Int) -> String = { it + 1 } // ERROR inferred type is Int but String was expected
    apply(1) { x, y -> x } // ERROR expected 1 parameter of type Int, but 2 were declared
    pair { it } // ERROR expected 2 parameters of types Int, Int, but 0 were declared
    val d: (String) -> Int = { s: Int -> s } // ERROR expected parameter of type String
    val n = 5
    n(1) // ERROR expression 'n' of type Int cannot be invoked as a function
    val f: ((Int) -> Int)? = null
    f(1) // ERROR reference has a nullable type '((Int) -> Int)?'
    val g = { x: Int -> x }
    g("s") // ERROR inferred type is String but Int was expected
    g(1, 2) // ERROR no function 'invoke' takes 2 arguments
    val h: () -> Unit = { return } // ERROR 'return' is not allowed here
    while (true) {
        run { break } // ERROR 'break' is not allowed outside a loop
    }
    val k: Int = run { "s" } // ERROR inferred type is String but Int was expected
    val p: (Int) -> Int = square // ERROR unresolved reference: square
    val q = apply(1) { "x" } // ERROR inferred type is String but Int was expected
    val r: () -> Long = { 1 }
    val s: () -> Int = { 1L } // ERROR inferred type is Long but Int was expected
    1(2) // ERROR expression of type Int cannot be invoked as a function
    each { return@nope } // ERROR unresolved label: nope
    each { x -> run { return@each } } // ERROR 'return@each' is not allowed here
    val t: Int = run { return@run "s" } // ERROR inferred type is String but Int was expected
    each { return@each 1 } // ERROR inferred type is Int but Unit was expected
    over { a, b, c -> a } // ERROR none of the 2 candidates for 'over' takes (lambda)
    withSb { this@nope.append(1) } // ERROR unresolved label: nope
    withSb { x -> append(x) } // ERROR expected no parameters, but 1 were declared
    val rr: StringBuilder.() -> Int = { length }
    rr(1) // ERROR type mismatch: inferred type is Int but StringBuilder was expected
    val bad: Int = rr // ERROR inferred type is StringBuilder.() -> Int but Int was expected
}

fun withSb(f: StringBuilder.() -> Unit) = f(StringBuilder())

fun over(f: (Int) -> Int) = f(1)
fun over(f: (Int, Int) -> Int) = f(1, 2)

fun each(f: (Int) -> Unit) = f(1)

class V<out T>(val f: (T) -> Unit) // ERROR type parameter T is declared as 'out' but occurs in 'in' position

fun destructured() {
    val f = { (a, b): Int -> a } // ERROR unresolved reference: component1
    val g = { (a, b) -> a } // ERROR cannot infer a type for this parameter
}

class Two(val a: Int, val b: Int) {
    operator fun component1() = a
    operator fun component2() = b
}

// A destructured name's declared type takes its component.
fun typedComponents(twos: Array<Two>) {
    val (x: String, y) = Two(1, 2) // ERROR inferred type is Int but String was expected
    val f = { (a: String, b): Two -> a } // ERROR inferred type is Int but String was expected
    for ((c, d: Long) in twos) {} // ERROR inferred type is Int but Long was expected
    val (e: Number, _) = Two(1, 2)
}

// FunctionN is the standard library's.
fun wide(f: (Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int, Int) -> Unit) {} // ERROR function types with more than 22 parameters need the Kotlin standard library

fun receiverCalls(f: (Int) -> Int, g: Int.(Int) -> Int) {
    2.(g)(1)
    2.(f)() // ERROR no receiver can be passed to a value of type (Int) -> Int
}

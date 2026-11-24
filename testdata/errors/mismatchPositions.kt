// Where kotlinc 2.4.21 reports a value that doesn't fit its expected type:
// at the start of an if, when or try whose branches don't fit, not at a
// branch.

fun doSmth() {}
fun c(): Boolean = true

fun ifValue(): Int =
    if (c()) { // ERROR inferred type is String but Int was expected
        1
    } else {
        ""
    }

fun whenValue(): Int =
    when { // ERROR inferred type is String but Int was expected
        c() -> 1
        else -> ""
    }

fun tryValue(): Int =
    try { // ERROR inferred type is Unit but Int was expected
        doSmth()
    } catch (e: Exception) {
        2
    }

fun returnedTry(): Int {
    return try { // ERROR inferred type is String but Int was expected
        1
    } catch (e: Exception) {
        ""
    }
}

fun finallyIgnored(): Int =
    try {
        1
    } catch (e: Exception) {
        2
    } finally {
        ""
    }

fun fits(): Any =
    if (c()) {
        1
    } else {
        ""
    }

// A multi-line value is reported at its first line.
class Node {
    fun next(): Node = this
    fun size(): Int = 1
}

fun chain(): String {
    return Node() // ERROR inferred type is Int but String was expected
        .next()
        .size()
}

val chained: String = Node() // ERROR inferred type is Int but String was expected
    .size()

// The condition in X, which takes the subject, is reported.
fun inCondition(s: String) =
    when (s) {
        in 1..10 -> 1 // ERROR inferred type is String but Int was expected
        else -> 2
    }

// An anonymous function returns its declared type, or Unit for a block
// body, and an expression body isn't coerced to Unit.
fun <T> call(f: () -> T): T = f()

fun anonBlockReturn() =
    call(fun() {
        return 42 // ERROR inferred type is Int but Unit was expected
    })

val anonExpr: () -> Unit = (fun() = "1") // ERROR inferred type is String but Unit was expected
val lambdaCoerced: () -> Unit = { "1" }

// A lambda in an annotation's argument ends its statements at newlines.
annotation class Anno(val n: Int)

@Anno({ // ERROR inferred type is () -> Int but Int was expected
    val x = 1
    x
})
fun annotated() {}

// A cast gives its type to a call of a function whose return type is its
// one type parameter, which its parameters don't have.
fun <T> make(): T = null!!
fun <V> id(v: V) = v

val fromCast = make() as String
val notThroughId = id(make()) as String // ERROR cannot infer type

class Sample

fun <K> id(x: K): K = x
fun takeLambda1(f: Sample.() -> Sample) {}
fun takeLambda2(f: Sample.(Sample) -> Sample) {}

val ok1: Sample.() -> Sample = { s -> s }
val ok2: Sample.(Sample) -> Sample = { s: Sample, t -> t }
val ok3: Sample.() -> Unit = id { s: Sample -> }
val ok4 = id<Sample.() -> Sample> { s -> s }

val bad1: Sample.() -> Sample = { s, t -> s } // ERROR expected no parameters, but 2 were declared

fun test() {
    takeLambda1 { s -> s } // ERROR expected no parameters, but 1 were declared
    takeLambda2 { s, t -> s } // ERROR expected 1 parameter of type Sample, but 2 were declared
    takeLambda2 { s -> s }
}

// Unlike a lambda's, an anonymous function's receiver isn't implicit,
// through a generic call too.
fun <K> idAnon(x: K): K = x

fun anonReceiver() {
    val a: Int.(String) -> Int = idAnon(fun (x: String) = 10) // ERROR type mismatch: inferred type is (String) -> Int but Int.(String) -> Int was expected
    val b: Int.(String) -> Int = idAnon(fun Int.(x: String) = 10)
    val c: Int.(String) -> Int = idAnon(fun (i: Int, x: String) = 10)
    val d: Int.(String) -> Int = idAnon { x: String -> 10 }
}

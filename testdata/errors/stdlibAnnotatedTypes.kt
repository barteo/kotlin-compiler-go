// On a type, an annotation takes arguments after a space unless an arrow
// follows them; the variable of a for loop and an anonymous function can
// be annotated.
@Target(AnnotationTarget.TYPE)
@Repeatable
annotation class Foo

annotation class Ann(val x: Int)

data class P(val x: Int, val y: Int)

val f1: @Foo () -> Unit = {}
val f2: @Foo () @Foo () -> Unit get() = {}
val f3: @Foo @Foo () () -> Unit = {}
val f4: @Foo@Foo () -> Unit = {}
val f6: @Foo (() -> Unit) = {}
val f7: @Foo (context(String) () -> Unit) = {}
fun f5(x: Any): @Foo() () -> Unit = x as @Foo () @[Foo Foo ()] @Foo() () -> Unit

fun loops(ps: Array<P>) {
    for (@Ann(1) i in 1..3) {}
    for (@Ann(2) (x, @Ann(3) y) in ps) {}
    for (@Err i in 1..3) {} // ERROR unresolved reference: Err
    val g = @Ann(4) fun() {}
    g()
}

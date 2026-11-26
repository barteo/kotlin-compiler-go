// A DSL marker on a function type marks its context parameters too.

@DslMarker
@Target(AnnotationTarget.CLASS, AnnotationTarget.TYPE)
annotation class Dsl
class A
@Dsl class B
context(_: A) fun needA() {}
fun f(block: @Dsl (context(A) () -> Unit)) {}
fun g(block: context(A) () -> Unit) {}
fun test() {
    f { needA(); with(B()) { needA() } } // ERROR cannot be called in this context with an implicit receiver
    g { needA(); with(B()) { needA() } }
}

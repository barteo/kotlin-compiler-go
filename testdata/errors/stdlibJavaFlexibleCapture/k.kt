import java.util.concurrent.CompletableFuture

abstract class B<S : CharSequence?> {
    abstract fun toFuture(): CompletableFuture<S>
}

// A Java F! read through the capture of out CharSequence? is a
// CharSequence!.
fun waitTest(res: B<*>) {
    C.await(res.toFuture()).length
    val i: Int = C.await(res.toFuture()) // ERROR inferred type is CharSequence! but Int was expected
}

interface Resolver<M1> {
    val exposeM: M1 get() = null!!
}

class ResolverImpl<M>(byModule: Map<M, String>, delegate: Resolver<M>) : Resolver<M>

interface WithFoo {
    fun foo()
}

// M of exact bounds M2! and M2? is M2!.
fun <M2 : WithFoo> exactTest(delegate: Resolver<M2?>): Resolver<M2?> {
    val byModule = MyMap<M2, String>()
    val result = ResolverImpl(byModule, delegate)
    result.exposeM.foo()
    return ResolverImpl(byModule, delegate)
}

// <T> @NotNull T: the annotation after the type parameters is the
// return type's.
fun notNullTest(s: String?) {
    val r: String = C.notNull(s)
}

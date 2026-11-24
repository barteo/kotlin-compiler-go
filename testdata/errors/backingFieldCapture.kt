// An explicit backing field read through a star-projected receiver is of
// the capture of the star, which that receiver's members take, as in K2,
// but another receiver's don't.
class C<E>(val value: E) {
    val num: Any?
        field: E = value

    fun consume(v: E) {}

    fun same(o: C<*>) {
        val it = o.num
        o.consume(it)
        o.consume(o.num)
    }

    fun other(a: C<*>, b: C<*>) {
        val it = b.num
        a.consume(it) // ERROR type mismatch
    }

    fun <R> generic(o: C<R>) {
        o.consume(o.num)
    }
}

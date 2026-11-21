package p

// Classes of the names lib2 declares too, whose own signatures name them:
// the class path's first ones, which a type in their members or bounds
// names while they load.
public class Box<T>(public val v: T) {
    public fun self(): Box<T> = this
}

public abstract class Node<N : Node<N>> {
    public abstract fun next(): N?
}

public class Leaf : Node<Leaf>() {
    override fun next(): Leaf? = null
}

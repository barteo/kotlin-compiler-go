fun interface IntOp {
    fun apply(a: Int, b: Int): Int
}

fun interface Handle {
    fun dispose()
}

interface Base {
    fun name(): String = "base"
    fun run(x: Int): String
}

// The abstract method can be inherited, and a subinterface can add bodies.
fun interface Runner : Base

fun interface Sub : Runner {
    override fun name() = "sub"
}

fun interface Mapper<T, R> {
    fun map(x: T): R
}

fun compute(op: IntOp, a: Int, b: Int) = op.apply(a, b)

class Registry {
    val handles = java.util.ArrayList<Handle>()
    fun add(h: Handle): Handle {
        handles.add(h)
        return h
    }
}

fun go(r: Runner) = r.name() + ":" + r.run(3)

fun <T, R> mapTwice(x: T, m: Mapper<T, R>, n: Mapper<R, R>): R = n.map(m.map(x))

class Holder(var op: IntOp)

fun main() {
    println(compute({ a, b -> a + b }, 2, 3))
    val mul = IntOp { a, b -> a * b }
    println(mul.apply(4, 5))
    var disposed = 0
    val reg = Registry()
    reg.add { disposed++ }
    reg.add(Handle { disposed += 10 })
    for (h in reg.handles) h.dispose()
    println(disposed)
    println(go { x -> "run$x" })
    val s = Sub { "s$it" }
    println(s.name() + " " + s.run(1))
    val o = object : IntOp {
        override fun apply(a: Int, b: Int) = a - b
    }
    println(compute(o, 9, 4))
    println(mapTwice(20, { it + 1 }) { it * 2 })
    val h = Holder { a, _ -> a }
    h.op = IntOp { _, b -> b }
    println(h.op.apply(1, 2))
    val ops: Array<IntOp> = arrayOf(IntOp { a, b -> a % b }, mul)
    println(ops[0].apply(7, 4) + ops[1].apply(2, 2))
}

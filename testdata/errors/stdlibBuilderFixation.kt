class Owner<T> {
    fun constrain(value: T) {}
    fun provide(): T = null!!
    val instance: T = null!!
}

fun <OT> pcla(lambda: (Owner<OT>) -> Unit): OT = null!!

interface Base

object Value

class Scope : Base {
    operator fun not(): Scope = this
    operator fun iterator(): Scope = this
    operator fun hasNext(): Boolean = false
    operator fun next(): Value = Value
    operator fun invoke(v: Value) {}
}

object Interloper : Base

fun notFixes() {
    pcla { o ->
        o.constrain(Scope())
        !o.provide()
        o.constrain(Interloper) // ERROR type mismatch
    }
}

fun forFixes() {
    pcla { o ->
        o.constrain(Scope())
        for (v in o.provide()) {}
        o.constrain(Interloper) // ERROR type mismatch
    }
}

fun invokeFixes() {
    pcla { o ->
        o.constrain(Scope())
        o.instance(Value)
        o.constrain(Interloper) // ERROR type mismatch
    }
}

class Buildee<TV> {
    fun set(value: TV) {}
    fun get(): TV = null!!
}

fun <PTV> parallelBuild(a: Buildee<PTV>.() -> Unit, b: Buildee<PTV>.() -> Unit): Buildee<PTV> = Buildee()

class Target
class Different

fun consumeDifferent(value: Different) {}

fun rejected() {
    parallelBuild(
        { set(Target()) },
        { consumeDifferent(get()) }, // ERROR type mismatch
    )
}

fun valid(flag: Boolean): List<Any> = buildList {
    add(flag)
    for (x in listOf(1, 2)) add(x)
    if (!get(0).equals(true)) add("no")
}

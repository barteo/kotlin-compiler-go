// Overloads with inferred return types that call one another, as
// kotlinpoet's CodeWriter.emitCode: resolving a call doesn't infer the
// return type of an overload that can't take it, which here is the
// function being inferred.
class Block(val parts: Array<out Any?>) {
    companion object {
        fun of(format: String, vararg args: Any?) = Block(arrayOf(format, *args))
    }
}

class Writer {
    val out = StringBuilder()

    fun emit(s: String) = emit(Block.of(s))

    fun emit(format: String, vararg args: Any?) = emit(Block.of(format, *args))

    fun emit(block: Block, newline: Boolean = false): Writer {
        for (p in block.parts) out.append(p).append(' ')
        if (newline) out.append('|')
        return this
    }
}

class Pair2<A, B>(val a: A, val b: B)

fun <T> wrap(t: T) = Pair2(t, wrap(t, 0))

fun <T> wrap(t: T, depth: Int): String = "$t@$depth"

fun main() {
    println(Writer().emit("a").emit("b %L", 1).emit(Block.of("c"), true).out)
    println(wrap("w").b)
}

package sample

class Stage {
    fun again(): Stage = this
    fun next(): Next = Next()
}
class End { fun number(): Int = 2 }
class Next {
    fun end(): End = End()
    fun finish(): Int = 1
}
fun consume(block: (Stage) -> Int): Int = block(Stage())
fun receive(block: Stage.() -> Int): Int = Stage().block()

fun hints(): Int = consume {
    it.again()
    receive {
        again()
        next().finish()
    }
}
fun explicit(): Int = consume { stage ->
    stage.again()
    stage.next().finish()
}
fun chain(): Int = Stage()
    .again()
    .again()
    .next()
    .end()
    .number()
fun value(): Int = consume {
    it.again()
    42
}
fun single(): Int = consume { 42 }
fun repeated(): Stage = Stage()
    .again()
    .again()
    .again()
fun <T> evaluate(block: () -> T): T = block()
fun result(): Int = evaluate {
    Stage()
    7
}
fun branch(flag: Boolean): Int = evaluate chosen@{
    if (flag) {
        10
    } else {
        20
    }
}
fun wrapped(): Int = (Stage()
    .again()
    .next())
    .end()
    .number()

// Which interfaces a lambda converts to (SAM conversion) when passed to a
// library's Kotlin functions: Java interfaces and fun interfaces, but not
// other Kotlin interfaces, as mockk's Answer.
package lib

interface Answer<out T> {
    fun answer(call: String): T
}

class AnswerScope(val call: String)

class Stub<T> {
    var how = ""

    infix fun answers(answer: Answer<T>): Stub<T> {
        how = "Answer " + answer.answer("a")
        return this
    }

    infix fun answers(answer: AnswerScope.(String) -> T): Stub<T> {
        how = "lambda " + AnswerScope("scope").answer("b")
        return this
    }
}

fun interface Check {
    fun check(x: Int): Boolean
}

fun countIf(xs: List<Int>, c: Check) = xs.count { c.check(it) }

fun runTwice(r: Runnable) {
    r.run()
    r.run()
}

fun <T> callIt(c: java.util.concurrent.Callable<T>): T = c.call()

package base

abstract class Base {
    protected fun hello(from: String) {
        println("hello from $from")
    }

    protected var greeting = "hello"
}

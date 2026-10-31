fun main() {
    lambdas { 3 }
    wrap { println("use wrapped $it") }
    println(suspending { 4 })
    callsInternal { "use" }
}

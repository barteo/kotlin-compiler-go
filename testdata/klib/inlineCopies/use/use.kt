import base.BaseTest
import base.twice

class MyTest : BaseTest() {
    fun run() = make<String>()
}

fun main() {
    println(MyTest().run())
    println(twice(5))
}

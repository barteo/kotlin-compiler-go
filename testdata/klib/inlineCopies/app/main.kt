import base.BaseTest
import base.twice

fun main() {
    println(BaseTest().make<Int>())
    println(BaseTest().makeWith<String> { items += "a" })
    println(twice(4))
}

import m.Items
import m.Node
import m.expectIs

fun main() {
    println(Items())
    println(Items(own = listOf(1)))
    println(Items(1..2, true))
    listOf<Node>(Node.Leaf(3), Node.Leaf(4)).forEachIndexed { i, n ->
        expectIs<Node.Leaf>(n)
        println("$i ${n.id}")
    }
}

package app

import Klass
import KlassAlias
import NestedAlias
import ObjektAlias
import Greeting
import topLevel
import topLevelVal

fun run() {
    val k: KlassAlias = KlassAlias()
    println(k.hello())
    println(NestedAlias().hello())
    println(ObjektAlias.hello())
    val g: Greeting = { "hi $it" }
    println(g("there"))
    println(topLevel() + " " + topLevelVal)
    println(Klass.Nested().hello())
}

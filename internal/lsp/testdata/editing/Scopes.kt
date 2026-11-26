package sample

class Scope { val editingMember: Int = 2 }
fun inScope(body: Scope.() -> Int): Int = Scope().body()
val editingGlobal: Int = 3

operator fun Scope.plus(n: Int): Int = editingMember + n
fun choose(n: Int): Int = n
fun choose(s: String): String = s

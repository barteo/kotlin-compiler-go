package sample

class Scope { val editingMember: Int = 2 }
fun inScope(body: Scope.() -> Int): Int = Scope().body()
val editingGlobal: Int = 3

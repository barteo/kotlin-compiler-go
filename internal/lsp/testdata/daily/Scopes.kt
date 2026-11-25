package sample

class Scope { val dailyMember: Int = 2 }
fun inScope(body: Scope.() -> Int): Int = Scope().body()
val dailyGlobal: Int = 3

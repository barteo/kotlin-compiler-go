// kotlinc can't see a class whose metadata says it is a Kotlin class, k = 1,
// but holds no data; it reads one whose k is left to its default as Java.

fun e(): String = E.OK.name // ERROR unresolved reference
fun f(): String = F.OK.name

fun sum(vararg xs: UInt): Int = xs.size
val s = sum(*intArrayOf(1)) // ERROR inferred type is IntArray but UIntArray was expected

val big: UByte = 300u // ERROR type mismatch

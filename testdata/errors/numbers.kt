fun main() {
    val f: Float = 1 // ERROR type mismatch: inferred type is Int but Float was expected
    val d: Float = 1.0 // ERROR type mismatch: inferred type is Double but Float was expected
    val b: Byte = 300 // ERROR type mismatch: inferred type is Int but Byte was expected
    var c: Byte = 1
    c += 1 // ERROR type mismatch: inferred type is Int but Byte was expected
    val x = 'a' + 'b' // ERROR operator '+' cannot be applied to Char and Char
    val y = 1 + 'a' // ERROR operator '+' cannot be applied to Int and Char
    val z: Char = 65 // ERROR type mismatch: inferred type is Int but Char was expected
    val n = -'a' // ERROR operator '-' cannot be applied to Char
    val r = 'a'..5 // ERROR type mismatch: inferred type is Int but Char was expected
    println(f + d + b + c + x + y + n)
    println(z)
    println(5 in 'a'..'z') // ERROR type mismatch: inferred type is Int but Char was expected
    val s: Short = 1
    println(s == 1.toByte()) // ERROR operator '==' cannot be applied to Short and Byte
    println(s.toChar()) // ERROR 'toChar' is deprecated
    println(1.5.toByte()) // ERROR 'toByte' is deprecated
}

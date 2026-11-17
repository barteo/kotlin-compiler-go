val bad = 1_ // ERROR illegal underscore
val badHex = 0x_f // ERROR illegal underscore
val badExp = 1.0e_1 // ERROR illegal underscore
val good = 1_000 + 0xFF_FF.toInt()
fun h() = run __@{ } // ERROR names _, __, ___, ... are reserved

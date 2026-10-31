import consts.*

annotation class Code(val value: Int)

// The library's consts are constants here too.
const val NEXT = MAX - 1
const val LABEL = "$RADIX/$DECEMBER"

@Code(Codes.FIRST_USER)
class Annotated

fun month(m: Int) = when (m) {
    DECEMBER -> "december"
    Holder.SHIFTED -> "shifted"
    else -> "other"
}

fun main() {
    println("$MAX $NEXT $LABEL $BYTE $BIG $PI $BUFFER $E")
    println("${Codes.FIRST_USER} ${Holder.SHIFTED}")
    println(Annotated::class.java.getAnnotation(Code::class.java).value)
    println("${month(12)} ${month(130)} ${month(1)}")
}

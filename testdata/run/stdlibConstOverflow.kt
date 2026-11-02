// Constant evaluation and primitive overflow: Char arithmetic wraps,
// Int-literal arithmetic overflows before widening to Long, unsigned
// literals are truncated, a const read through its object does not
// initialize it, constant templates are folded into one interned
// constant, and folded String constants longer than the class file
// limit (65535 bytes of modified UTF-8) are still built correctly.

object Holder {
    const val K = 42
    init {
        println("Holder initialized")
    }
}

const val S0 = "0123456789abcdef"
const val S1 = S0 + S0
const val S2 = S1 + S1
const val S3 = S2 + S2
const val S4 = S3 + S3
const val S5 = S4 + S4
const val S6 = S5 + S5
const val S7 = S6 + S6
const val S8 = S7 + S7
const val S9 = S8 + S8
const val S10 = S9 + S9
const val S11 = S10 + S10

const val E0 = "€€€€€€€€"
const val E1 = E0 + E0
const val E2 = E1 + E1
const val E3 = E2 + E2
const val E4 = E3 + E3
const val E5 = E4 + E4
const val E6 = E5 + E5
const val E7 = E6 + E6
const val E8 = E7 + E7
const val E9 = E8 + E8
const val E10 = E9 + E9
const val E11 = E10 + E10

const val NAME = "kc"
const val TEMPLATE = "hello ${NAME} ${1 + 2} ${'c'} ${true} ${null}"

fun main() {
    val c: Char = Char.MAX_VALUE + 1
    println(c.code)
    val d: Char = Char.MIN_VALUE - 1
    println(d.code)

    val l: Long = 2147483647 + 1
    println(l)
    val m: Long = 1000000 * 1000000
    println(m)
    val n: Long = -2147483647 - 2
    println(n)

    val ub: UByte = 255u
    println(ub)
    val us: UShort = 65535u
    println(us)

    println(Holder.K)
    println("after K")

    println(TEMPLATE)
    println(TEMPLATE === "hello kc 3 c true null")

    val big = S11 + S11 + "!"
    println(big.length)
    println(big.substring(big.length - 3))
    val euros = E11 + E11
    println(euros.length)
    println(euros.all { it == '€' })
}

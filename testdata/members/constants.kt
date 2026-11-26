package members

object Constants {
    const val F = 5f
    const val F_PLUS = F + 1f
    const val F_TIMES = 2f * 3.5f
    const val F_INT = 5f + 1
    const val F_DIV = 1f / 3f
    const val F_REM = 7.5f % 2f
    const val F_ZERO_DIV = 1f / 0f
    const val F_NAN = 0f / 0f
    const val D = 1.5 + 2.0
    const val D_INT = 1.5 * 2
    const val D_DIV = 1.0 / 3
    const val D_REM = -7.5 % 2.0
    const val D_NEG_ZERO = -0.0 * 1.0
    const val L_TO_D = Long.MAX_VALUE.toDouble()
    const val I_TO_F = 16777217.toFloat()
    const val F_TO_I = F.toInt() + 1
    const val D_TO_I = 1e10.toInt()
    const val D_TO_L = -1e30.toLong()
    const val NAN_TO_I = F_NAN.toInt()
    const val C = 'a' + 1
    const val C_MINUS = 'z' - 2
    const val NOT = !true
    const val AND = true && !false
    const val OR = false || NOT
    const val LESS = 1 < 2
    const val F_LESS = F < 4.5
    const val NAN_EQ = F_NAN == F_NAN
    const val C_GREATER = 'b' > 'a'
    const val NEG_ZERO_EQ = D_NEG_ZERO == 0.0
    const val NEG_ZERO_LESS = D_NEG_ZERO < 0.0
    const val NAN_NE = F_NAN != F_NAN
    const val NAN_GREATER = F_NAN > 1f
    const val NAN_LESS = F_NAN < 1f
    const val S_F = "x" + F
    const val S_D = "d=$D"
    const val S_SMALL = "" + 0.001
    const val S_SMALLER = "" + 0.0001f
    const val S_BIG = "" + 1e7
    const val S_BIGGER = "" + 123456789.0f
    const val S_THIRD = "" + D_DIV
    const val S_NEG = "" + (-1.25e-5)
    const val S_INF = "" + F_ZERO_DIV
    const val S_NAN = "" + F_NAN
    const val S_NEG_ZERO = "" + D_NEG_ZERO
    const val S_BOOL = "b" + LESS
}

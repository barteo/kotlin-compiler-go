// kotlinc: -jvm-target 17
// Functions and constructors with more than 32 parameters with default
// values, compiled by kotlinc: their $default methods take an int of
// masks for every 32 parameters, the context parameters counted first.
package lib

fun top40(
    p0: Int = 1, p1: Int = 2, p2: Int = 3, p3: Int = 4, p4: Int = 5, p5: Int = 6,
    p6: Int = 7, p7: Int = 8, p8: Int = 9, p9: Int = 10, p10: Int = 11, p11: Int = 12,
    p12: Int = 13, p13: Int = 14, p14: Int = 15, p15: Int = 16, p16: Int = 17, p17: Int = 18,
    p18: Int = 19, p19: Int = 20, p20: Int = 21, p21: Int = 22, p22: Int = 23, p23: Int = 24,
    p24: Int = 25, p25: Int = 26, p26: Int = 27, p27: Int = 28, p28: Int = 29, p29: Int = 30,
    p30: Int = 31, p31: Int = 32, p32: Int = 33, p33: Int = 34, p34: Int = 35, p35: Int = 36,
    p36: Int = 37, p37: Int = 38, p38: Int = 39, p39: Int = 40,
): String = "" + p0 + " " + p32 + " " + p39 + " " + (
        p0 + p1 + p2 + p3 + p4 + p5 + p6 + p7 + p8 + p9 + p10 + p11 +
        p12 + p13 + p14 + p15 + p16 + p17 + p18 + p19 + p20 + p21 + p22 + p23 +
        p24 + p25 + p26 + p27 + p28 + p29 + p30 + p31 + p32 + p33 + p34 + p35 +
        p36 + p37 + p38 + p39
    )

inline fun inline35(
    p0: Int = 1, p1: Int = 2, p2: Int = 3, p3: Int = 4, p4: Int = 5, p5: Int = 6,
    p6: Int = 7, p7: Int = 8, p8: Int = 9, p9: Int = 10, p10: Int = 11, p11: Int = 12,
    p12: Int = 13, p13: Int = 14, p14: Int = 15, p15: Int = 16, p16: Int = 17, p17: Int = 18,
    p18: Int = 19, p19: Int = 20, p20: Int = 21, p21: Int = 22, p22: Int = 23, p23: Int = 24,
    p24: Int = 25, p25: Int = 26, p26: Int = 27, p27: Int = 28, p28: Int = 29, p29: Int = 30,
    p30: Int = 31, p31: Int = 32, p32: Int = 33, p33: Int = 34, p34: Int = 35,
): String = "" + p0 + " " + p34 + " " + (
        p0 + p1 + p2 + p3 + p4 + p5 + p6 + p7 + p8 + p9 + p10 + p11 +
        p12 + p13 + p14 + p15 + p16 + p17 + p18 + p19 + p20 + p21 + p22 + p23 +
        p24 + p25 + p26 + p27 + p28 + p29 + p30 + p31 + p32 + p33 + p34
    )

context(c: String)
fun ctx32(
    p0: Int = 1, p1: Int = 2, p2: Int = 3, p3: Int = 4, p4: Int = 5, p5: Int = 6,
    p6: Int = 7, p7: Int = 8, p8: Int = 9, p9: Int = 10, p10: Int = 11, p11: Int = 12,
    p12: Int = 13, p13: Int = 14, p14: Int = 15, p15: Int = 16, p16: Int = 17, p17: Int = 18,
    p18: Int = 19, p19: Int = 20, p20: Int = 21, p21: Int = 22, p22: Int = 23, p23: Int = 24,
    p24: Int = 25, p25: Int = 26, p26: Int = 27, p27: Int = 28, p28: Int = 29, p29: Int = 30,
    p30: Int = 31, p31: Int = 32,
): String = c + " " + p0 + " " + p31 + " " + (
        p0 + p1 + p2 + p3 + p4 + p5 + p6 + p7 + p8 + p9 + p10 + p11 +
        p12 + p13 + p14 + p15 + p16 + p17 + p18 + p19 + p20 + p21 + p22 + p23 +
        p24 + p25 + p26 + p27 + p28 + p29 + p30 + p31
    )

fun Int.ext33(
    p0: Int = 1, p1: Int = 2, p2: Int = 3, p3: Int = 4, p4: Int = 5, p5: Int = 6,
    p6: Int = 7, p7: Int = 8, p8: Int = 9, p9: Int = 10, p10: Int = 11, p11: Int = 12,
    p12: Int = 13, p13: Int = 14, p14: Int = 15, p15: Int = 16, p16: Int = 17, p17: Int = 18,
    p18: Int = 19, p19: Int = 20, p20: Int = 21, p21: Int = 22, p22: Int = 23, p23: Int = 24,
    p24: Int = 25, p25: Int = 26, p26: Int = 27, p27: Int = 28, p28: Int = 29, p29: Int = 30,
    p30: Int = 31, p31: Int = 32, p32: Int = 33,
): String = "" + this + " " + p32 + " " + (
        p0 + p1 + p2 + p3 + p4 + p5 + p6 + p7 + p8 + p9 + p10 + p11 +
        p12 + p13 + p14 + p15 + p16 + p17 + p18 + p19 + p20 + p21 + p22 + p23 +
        p24 + p25 + p26 + p27 + p28 + p29 + p30 + p31 + p32
    )

class Big(val k: Int) {
    fun m66(
        p0: Int = 1, p1: Int = 2, p2: Int = 3, p3: Int = 4, p4: Int = 5, p5: Int = 6,
        p6: Int = 7, p7: Int = 8, p8: Int = 9, p9: Int = 10, p10: Int = 11, p11: Int = 12,
        p12: Int = 13, p13: Int = 14, p14: Int = 15, p15: Int = 16, p16: Int = 17, p17: Int = 18,
        p18: Int = 19, p19: Int = 20, p20: Int = 21, p21: Int = 22, p22: Int = 23, p23: Int = 24,
        p24: Int = 25, p25: Int = 26, p26: Int = 27, p27: Int = 28, p28: Int = 29, p29: Int = 30,
        p30: Int = 31, p31: Int = 32, p32: Int = 33, p33: Int = 34, p34: Int = 35, p35: Int = 36,
        p36: Int = 37, p37: Int = 38, p38: Int = 39, p39: Int = 40, p40: Int = 41, p41: Int = 42,
        p42: Int = 43, p43: Int = 44, p44: Int = 45, p45: Int = 46, p46: Int = 47, p47: Int = 48,
        p48: Int = 49, p49: Int = 50, p50: Int = 51, p51: Int = 52, p52: Int = 53, p53: Int = 54,
        p54: Int = 55, p55: Int = 56, p56: Int = 57, p57: Int = 58, p58: Int = 59, p59: Int = 60,
        p60: Int = 61, p61: Int = 62, p62: Int = 63, p63: Int = 64, p64: Int = 65, p65: Int = 66,
    ): String = "" + k + " " + p1 + " " + p65 + " " + (
            p0 + p1 + p2 + p3 + p4 + p5 + p6 + p7 + p8 + p9 + p10 + p11 +
            p12 + p13 + p14 + p15 + p16 + p17 + p18 + p19 + p20 + p21 + p22 + p23 +
            p24 + p25 + p26 + p27 + p28 + p29 + p30 + p31 + p32 + p33 + p34 + p35 +
            p36 + p37 + p38 + p39 + p40 + p41 + p42 + p43 + p44 + p45 + p46 + p47 +
            p48 + p49 + p50 + p51 + p52 + p53 + p54 + p55 + p56 + p57 + p58 + p59 +
            p60 + p61 + p62 + p63 + p64 + p65
        )
}

class C34(
    val p0: Int = 0, val p1: Int = 1, val p2: Int = 2, val p3: Int = 3, val p4: Int = 4, val p5: Int = 5,
    val p6: Int = 6, val p7: Int = 7, val p8: Int = 8, val p9: Int = 9, val p10: Int = 10, val p11: Int = 11,
    val p12: Int = 12, val p13: Int = 13, val p14: Int = 14, val p15: Int = 15, val p16: Int = 16, val p17: Int = 17,
    val p18: Int = 18, val p19: Int = 19, val p20: Int = 20, val p21: Int = 21, val p22: Int = 22, val p23: Int = 23,
    val p24: Int = 24, val p25: Int = 25, val p26: Int = 26, val p27: Int = 27, val p28: Int = 28, val p29: Int = 29,
    val p30: Int = 30, val p31: Int = 31, val p32: Int = 32, val p33: Int = 33,
) {
    override fun toString() = "" + p0 + " " + p33
}

data class D40(
    val p0: String = "d0", val p1: String = "d1", val p2: String = "d2", val p3: String = "d3", val p4: String = "d4",
    val p5: String = "d5", val p6: String = "d6", val p7: String = "d7", val p8: String = "d8", val p9: String = "d9",
    val p10: String = "d10", val p11: String = "d11", val p12: String = "d12", val p13: String = "d13", val p14: String = "d14",
    val p15: String = "d15", val p16: String = "d16", val p17: String = "d17", val p18: String = "d18", val p19: String = "d19",
    val p20: String = "d20", val p21: String = "d21", val p22: String = "d22", val p23: String = "d23", val p24: String = "d24",
    val p25: String = "d25", val p26: String = "d26", val p27: String = "d27", val p28: String = "d28", val p29: String = "d29",
    val p30: String = "d30", val p31: String = "d31", val p32: String = "d32", val p33: String = "d33", val p34: String = "d34",
    val p35: String = "d35", val p36: String = "d36", val p37: String = "d37", val p38: String = "d38", val p39: String = "d39",
)

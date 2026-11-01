// Default arguments of functions and constructors with more than 32 and
// 64 parameters: kotlinc's $default methods take an int of the masks of
// the omitted ones for every 32.

fun f33(
    p0: Int = 1, p1: Int = 2, p2: Int = 3, p3: Int = 4, p4: Int = 5, p5: Int = 6,
    p6: Int = 7, p7: Int = 8, p8: Int = 9, p9: Int = 10, p10: Int = 11, p11: Int = 12,
    p12: Int = 13, p13: Int = 14, p14: Int = 15, p15: Int = 16, p16: Int = 17, p17: Int = 18,
    p18: Int = 19, p19: Int = 20, p20: Int = 21, p21: Int = 22, p22: Int = 23, p23: Int = 24,
    p24: Int = 25, p25: Int = 26, p26: Int = 27, p27: Int = 28, p28: Int = 29, p29: Int = 30,
    p30: Int = 31, p31: Int = 32, p32: Int = 33,
): String = "" + p0 + " " + p31 + " " + p32 + " " + (
        p0 + p1 + p2 + p3 + p4 + p5 + p6 + p7 + p8 + p9 + p10 + p11 +
        p12 + p13 + p14 + p15 + p16 + p17 + p18 + p19 + p20 + p21 + p22 + p23 +
        p24 + p25 + p26 + p27 + p28 + p29 + p30 + p31 + p32
    )

fun f70(
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
    p66: Int = 67, p67: Int = 68, p68: Int = 69, p69: Int = 70,
): String = "" + p0 + " " + p31 + " " + p32 + " " + p63 + " " + p64 + " " + p69 + " " + (
        p0 + p1 + p2 + p3 + p4 + p5 + p6 + p7 + p8 + p9 + p10 + p11 +
        p12 + p13 + p14 + p15 + p16 + p17 + p18 + p19 + p20 + p21 + p22 + p23 +
        p24 + p25 + p26 + p27 + p28 + p29 + p30 + p31 + p32 + p33 + p34 + p35 +
        p36 + p37 + p38 + p39 + p40 + p41 + p42 + p43 + p44 + p45 + p46 + p47 +
        p48 + p49 + p50 + p51 + p52 + p53 + p54 + p55 + p56 + p57 + p58 + p59 +
        p60 + p61 + p62 + p63 + p64 + p65 + p66 + p67 + p68 + p69
    )

fun String.ext40(
    p0: Int = 1, p1: Int = 2, p2: Int = 3, p3: Int = 4, p4: Int = 5, p5: Int = 6,
    p6: Int = 7, p7: Int = 8, p8: Int = 9, p9: Int = 10, p10: Int = 11, p11: Int = 12,
    p12: Int = 13, p13: Int = 14, p14: Int = 15, p15: Int = 16, p16: Int = 17, p17: Int = 18,
    p18: Int = 19, p19: Int = 20, p20: Int = 21, p21: Int = 22, p22: Int = 23, p23: Int = 24,
    p24: Int = 25, p25: Int = 26, p26: Int = 27, p27: Int = 28, p28: Int = 29, p29: Int = 30,
    p30: Int = 31, p31: Int = 32, p32: Int = 33, p33: Int = 34, p34: Int = 35, p35: Int = 36,
    p36: Int = 37, p37: Int = 38, p38: Int = 39, p39: Int = 40,
): String = this + " " + p0 + " " + p39 + " " + (
        p0 + p1 + p2 + p3 + p4 + p5 + p6 + p7 + p8 + p9 + p10 + p11 +
        p12 + p13 + p14 + p15 + p16 + p17 + p18 + p19 + p20 + p21 + p22 + p23 +
        p24 + p25 + p26 + p27 + p28 + p29 + p30 + p31 + p32 + p33 + p34 + p35 +
        p36 + p37 + p38 + p39
    )

class M(val k: Int) {
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

class C35(
    val p0: Int = 0, val p1: Int = 1, val p2: Int = 2, val p3: Int = 3, val p4: Int = 4, val p5: Int = 5,
    val p6: Int = 6, val p7: Int = 7, val p8: Int = 8, val p9: Int = 9, val p10: Int = 10, val p11: Int = 11,
    val p12: Int = 12, val p13: Int = 13, val p14: Int = 14, val p15: Int = 15, val p16: Int = 16, val p17: Int = 17,
    val p18: Int = 18, val p19: Int = 19, val p20: Int = 20, val p21: Int = 21, val p22: Int = 22, val p23: Int = 23,
    val p24: Int = 24, val p25: Int = 25, val p26: Int = 26, val p27: Int = 27, val p28: Int = 28, val p29: Int = 29,
    val p30: Int = 30, val p31: Int = 31, val p32: Int = 32, val p33: Int = 33, val p34: Int = 34,
) {
    override fun toString() = "" + p0 + " " + p32 + " " + p34
}

data class D70(
    val p0: String = "d0", val p1: String = "d1", val p2: String = "d2", val p3: String = "d3", val p4: String = "d4",
    val p5: String = "d5", val p6: String = "d6", val p7: String = "d7", val p8: String = "d8", val p9: String = "d9",
    val p10: String = "d10", val p11: String = "d11", val p12: String = "d12", val p13: String = "d13", val p14: String = "d14",
    val p15: String = "d15", val p16: String = "d16", val p17: String = "d17", val p18: String = "d18", val p19: String = "d19",
    val p20: String = "d20", val p21: String = "d21", val p22: String = "d22", val p23: String = "d23", val p24: String = "d24",
    val p25: String = "d25", val p26: String = "d26", val p27: String = "d27", val p28: String = "d28", val p29: String = "d29",
    val p30: String = "d30", val p31: String = "d31", val p32: String = "d32", val p33: String = "d33", val p34: String = "d34",
    val p35: String = "d35", val p36: String = "d36", val p37: String = "d37", val p38: String = "d38", val p39: String = "d39",
    val p40: String = "d40", val p41: String = "d41", val p42: String = "d42", val p43: String = "d43", val p44: String = "d44",
    val p45: String = "d45", val p46: String = "d46", val p47: String = "d47", val p48: String = "d48", val p49: String = "d49",
    val p50: String = "d50", val p51: String = "d51", val p52: String = "d52", val p53: String = "d53", val p54: String = "d54",
    val p55: String = "d55", val p56: String = "d56", val p57: String = "d57", val p58: String = "d58", val p59: String = "d59",
    val p60: String = "d60", val p61: String = "d61", val p62: String = "d62", val p63: String = "d63", val p64: String = "d64",
    val p65: String = "d65", val p66: String = "d66", val p67: String = "d67", val p68: String = "d68", val p69: String = "d69",
)

class J34 @JvmOverloads constructor(
    val p0: Int = 0, val p1: Int = 1, val p2: Int = 2, val p3: Int = 3, val p4: Int = 4, val p5: Int = 5,
    val p6: Int = 6, val p7: Int = 7, val p8: Int = 8, val p9: Int = 9, val p10: Int = 10, val p11: Int = 11,
    val p12: Int = 12, val p13: Int = 13, val p14: Int = 14, val p15: Int = 15, val p16: Int = 16, val p17: Int = 17,
    val p18: Int = 18, val p19: Int = 19, val p20: Int = 20, val p21: Int = 21, val p22: Int = 22, val p23: Int = 23,
    val p24: Int = 24, val p25: Int = 25, val p26: Int = 26, val p27: Int = 27, val p28: Int = 28, val p29: Int = 29,
    val p30: Int = 30, val p31: Int = 31, val p32: Int = 32, val p33: Int = 33,
) {
    override fun toString() = "" + p0 + " " + p33
}

fun main() {
    println(f33())
    println(f33(p32 = 100))
    println(f33(p0 = 7, p31 = 8))
    println(f70())
    println(f70(p64 = -1, p69 = -2))
    println(f70(p0 = 5, p32 = 6, p63 = 7))
    println("x".ext40(p39 = 0))
    println(M(3).m66(p65 = 0, p1 = 9))
    println(C35())
    println(C35(p34 = 99))
    val d = D70(p0 = "a")
    println("" + d.p0 + " " + d.p31 + " " + d.p64 + " " + d.p69)
    val e = d.copy(p33 = "b", p68 = "c")
    println("" + e.p0 + " " + e.p33 + " " + e.p64 + " " + e.p68 + " " + e.p69)
    println(J34())
    println(J34(1))
}

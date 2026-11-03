import lib.*

// The program's own, which kotlinc calls from use/.
fun app70(
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
): String = "" + p0 + " " + p64 + " " + p69 + " " + (
        p0 + p1 + p2 + p3 + p4 + p5 + p6 + p7 + p8 + p9 + p10 + p11 +
        p12 + p13 + p14 + p15 + p16 + p17 + p18 + p19 + p20 + p21 + p22 + p23 +
        p24 + p25 + p26 + p27 + p28 + p29 + p30 + p31 + p32 + p33 + p34 + p35 +
        p36 + p37 + p38 + p39 + p40 + p41 + p42 + p43 + p44 + p45 + p46 + p47 +
        p48 + p49 + p50 + p51 + p52 + p53 + p54 + p55 + p56 + p57 + p58 + p59 +
        p60 + p61 + p62 + p63 + p64 + p65 + p66 + p67 + p68 + p69
    )

fun main() {
    println(top40())
    println(top40(p32 = 0, p39 = 0))
    println(inline35())
    println(inline35(p34 = 0))
    with("ctx") {
        println(ctx32())
        println(ctx32(p31 = 0))
    }
    println(7.ext33(p32 = 0))
    println(Big(2).m66(p65 = 0))
    println(C34())
    println(C34(p33 = 1))
    val d = D40().copy(p35 = "x")
    println(d.p0 + " " + d.p35 + " " + d.p39)
    println(app70(p64 = 0))
}

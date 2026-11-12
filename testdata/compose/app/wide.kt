package app

import androidx.compose.runtime.Composable
import lib.Runs
import lib.Text
import lib.WideLib

// A composable with default values and more than 31 parameters: it takes
// two ints of masks of the omitted arguments, and four of change flags.
@Composable
fun Wide(
    label: String,
    p0: Int = 0,
    p1: Int = 1,
    p2: Int = 2,
    p3: Int = 3,
    p4: Int = 4,
    p5: Int = 5,
    p6: Int = 6,
    p7: Int = 7,
    p8: Int = 8,
    p9: Int = 9,
    p10: Int = 10,
    p11: Int = 11,
    p12: Int = 12,
    p13: Int = 13,
    p14: Int = 14,
    p15: Int = 15,
    p16: Int = 16,
    p17: Int = 17,
    p18: Int = 18,
    p19: Int = 19,
    p20: Int = 20,
    p21: Int = 21,
    p22: Int = 22,
    p23: Int = 23,
    p24: Int = 24,
    p25: Int = 25,
    p26: Int = 26,
    p27: Int = 27,
    p28: Int = 28,
    p29: Int = 29,
    p30: Int = 30,
    p31: Int = 31,
    p32: Int = 32,
    last: String = "end",
) {
    Runs.hit("wide $label")
    Text("$label p0=$p0 p30=$p30 p31=$p31 p32=$p32 sum=${listOf(p0, p1, p2, p3, p4, p5, p6, p7, p8, p9, p10, p11, p12, p13, p14, p15, p16, p17, p18, p19, p20, p21, p22, p23, p24, p25, p26, p27, p28, p29, p30, p31, p32).sum()} $last")
}

// An open class whose open composable has default values and more than
// 31 parameters, which its name$default method takes the masks of.
open class WidePanel(val name: String) {
    @Composable
    open fun Show(
        p0: Int = 0,
        p1: Int = 1,
        p2: Int = 2,
        p3: Int = 3,
        p4: Int = 4,
        p5: Int = 5,
        p6: Int = 6,
        p7: Int = 7,
        p8: Int = 8,
        p9: Int = 9,
        p10: Int = 10,
        p11: Int = 11,
        p12: Int = 12,
        p13: Int = 13,
        p14: Int = 14,
        p15: Int = 15,
        p16: Int = 16,
        p17: Int = 17,
        p18: Int = 18,
        p19: Int = 19,
        p20: Int = 20,
        p21: Int = 21,
        p22: Int = 22,
        p23: Int = 23,
        p24: Int = 24,
        p25: Int = 25,
        p26: Int = 26,
        p27: Int = 27,
        p28: Int = 28,
        p29: Int = 29,
        p30: Int = 30,
        p31: Int = 31,
        p32: Int = 32,
        tail: String = name,
    ) {
        Text("panel $name p0=$p0 p31=$p31 p32=$p32 sum=${listOf(p0, p1, p2, p3, p4, p5, p6, p7, p8, p9, p10, p11, p12, p13, p14, p15, p16, p17, p18, p19, p20, p21, p22, p23, p24, p25, p26, p27, p28, p29, p30, p31, p32).sum()} $tail")
    }
}

class LoudPanel : WidePanel("loud") {
    @Composable
    override fun Show(
        p0: Int,
        p1: Int,
        p2: Int,
        p3: Int,
        p4: Int,
        p5: Int,
        p6: Int,
        p7: Int,
        p8: Int,
        p9: Int,
        p10: Int,
        p11: Int,
        p12: Int,
        p13: Int,
        p14: Int,
        p15: Int,
        p16: Int,
        p17: Int,
        p18: Int,
        p19: Int,
        p20: Int,
        p21: Int,
        p22: Int,
        p23: Int,
        p24: Int,
        p25: Int,
        p26: Int,
        p27: Int,
        p28: Int,
        p29: Int,
        p30: Int,
        p31: Int,
        p32: Int,
        tail: String,
    ) {
        Text("LOUD p1=$p1 p32=$p32 $tail")
    }
}

// Calls of the wide composables, the program's and kotlinc's, with some
// arguments omitted, and none.
@Composable
fun Wides(n: Int) {
    Wide("app")
    Wide("app n", p0 = n, p31 = n, last = "!")
    Wide("app all", 0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 30, 32, 34, 36, 38, 40, 42, 44, 46, 48, 50, 52, 54, 56, 58, 60, 62, 64, "all")
    WidePanel("p").Show(p32 = n)
    LoudPanel().Show(p1 = n)
    WideLib("lib from app", p32 = n)
}

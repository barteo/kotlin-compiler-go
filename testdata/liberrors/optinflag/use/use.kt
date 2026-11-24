// kotlinc: -opt-in=lib.Exp
import lib.*

fun f1() = expFun()
fun f2() = otherFun() // ERROR needs opt-in

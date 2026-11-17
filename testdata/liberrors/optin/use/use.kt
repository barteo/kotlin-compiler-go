import lib.*

fun f1() = expFun() // ERROR needs opt-in
fun f2() = softFun()
fun f3() = expVal // ERROR needs opt-in
fun f4() { expVar = 2 } // ERROR needs opt-in
fun f5(): ExpClass? = null // ERROR needs opt-in
fun f6(e: ExpClass.Nested) = e // ERROR needs opt-in
fun f7() = Holder(1) // ERROR needs opt-in
fun f8() = Holder()
fun f9(h: Holder) = h.prop // ERROR needs opt-in
fun f10(h: Holder) = h.both // ERROR needs opt-in
fun f11(h: Holder) = h.setterOnly
fun f12(h: Holder) { h.setterOnly = 2 } // ERROR needs opt-in
fun f13(): ExpAlias = "" // ERROR needs opt-in
fun f14() = 1.ext() // ERROR needs opt-in

class Sub : Sealedish() // ERROR needs opt-in to inherit

@OptIn(Exp::class)
class Sub2 : Sealedish()

@SubclassOptInRequired(Exp::class)
open class Sub3 : Sealedish()

class Over : Base() {
    override fun hook() = 2 // ERROR this override needs opt-in
}

@OptIn(Exp::class)
fun ok() = expFun() + expVal + ExpClass().member() + Holder(1).prop + 1.ext()

@Exp
fun propagated() = expFun()

fun usesPropagated() = propagated() // ERROR needs opt-in

fun components(d: Data) {
    val (x, y) = d // ERROR needs opt-in
    d.component1() // ERROR needs opt-in
    d.component2()
}

import lib.*

fun f1() = errFun() // ERROR 'errFun' is deprecated with level ERROR
fun f2() = hiddenFun() // ERROR unresolved reference
fun f3() = warnFun()
fun f4(): ErrClass? = null // ERROR 'ErrClass' is deprecated with level ERROR
fun f5() = errVal // ERROR 'errVal' is deprecated with level ERROR
fun f6(): ErrAlias = "" // ERROR 'ErrAlias' is deprecated with level ERROR
fun f7() = Holder("") // ERROR deprecated with level ERROR
fun f8() = Holder(1)
fun f9(h: Holder) = h.prop // ERROR 'prop' is deprecated with level ERROR
fun f10(h: Holder) = h.member() // ERROR 'member' is deprecated with level ERROR
fun f11() = JavaLib.errMethod() // ERROR 'errMethod' is deprecated with level ERROR
fun f12() = JavaLib.javaDeprecated()
fun f13() = JavaLib.hiddenMethod() // ERROR unresolved reference

@Suppress("DEPRECATION_ERROR")
fun suppressed() = errFun()

fun h1(): HiddenClass? = null // ERROR deprecated with level HIDDEN
fun h2() = HiddenClass() // ERROR deprecated with level HIDDEN
fun h3(): HiddenAlias = "" // ERROR deprecated with level HIDDEN
fun h4(): Box.HiddenNested? = null // ERROR deprecated with level HIDDEN
fun h5() = HiddenJava.f() // ERROR deprecated with level HIDDEN
fun h6(): lib.HiddenClass? = null // ERROR deprecated with level HIDDEN

// A library's internal objects, which another module can't use as values,
// as kotlinc's INVISIBLE_REFERENCE.

import a.*

fun test() {
    PublicClass.Companion // ERROR class PublicClass.Companion is not public
    PublicClass.Obj // ERROR class PublicClass.Obj is not public
    val c = PublicClass // ERROR class PublicClass.Companion is not public
    PublicClass.PublicObj
    WithCompanion.Companion
    val w = WithCompanion
}

// PROTECTED_CALL_FROM_PUBLIC_INLINE_ERROR: a public-API inline function
// can't use the protected members of a Java class, static ones too.

import p.Base

class K : Base() {
    inline fun a() = field // ERROR protected
    inline fun b() = STATIC // ERROR protected
    inline fun c() = stat() // ERROR protected
    inline fun d() = inst() // ERROR protected
    protected inline fun e() = field + STATIC + stat() + inst()
    private inline fun f() = field + STATIC + stat() + inst()
    fun g() = field + STATIC + stat() + inst()
}

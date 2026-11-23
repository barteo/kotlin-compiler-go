// A local class in an inline function is reported with its functions,
// and its code is still checked: a call of the function's lambda in it
// would return from the function.

inline fun <R> inlineFun(p: () -> R) {
    class A { // ERROR local classes are not yet supported in inline functions
        val z = p() // ERROR may contain non-local returns
        fun a() { // ERROR not yet supported in inline functions
            p() // ERROR may contain non-local returns
        }
    }
}

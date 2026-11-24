// Public-API inline functions and accessors can't use what other modules
// can't: the members of a @PublishedApi internal class's inline ones
// included, and default values and constructors of non-public classes.
@PublishedApi
internal class A {
    @PublishedApi
    internal fun test() {
        internalFun()
    }

    @PublishedApi
    internal inline fun testInline() {
        publicFun()
        internalFun() // ERROR public-API inline function cannot access non-public-API function 'internalFun'
    }

    @PublishedApi
    internal inline var publishedVar: Int
        get() {
            internalFun() // ERROR public-API inline function cannot access non-public-API function 'internalFun'
            return 1
        }
        set(value) {
            privateFun() // ERROR public-API inline function cannot access non-public-API function 'privateFun'
        }
}

inline var inlineVar: Int
    get() {
        A().test()
        return 1
    }
    set(value) {}

val b: Int
    inline get() {
        internalFun() // ERROR public-API inline function cannot access non-public-API function 'internalFun'
        return 1
    }

internal class Internal {
    fun f() {}
}

inline fun defaults(g: () -> Unit = ::internalFun) { // ERROR non-public-API function 'internalFun'
    Internal().f() // ERROR non-public-API class 'Internal'
}

fun publicFun() {}
internal fun internalFun() {}
private fun privateFun() {}

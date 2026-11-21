package a

class X

enum class E { SOME }

class Some {
    companion object {
        val foo = 1
    }
}

@Deprecated("Use b.H", level = DeprecationLevel.HIDDEN)
class H {
    fun m1() {}
}

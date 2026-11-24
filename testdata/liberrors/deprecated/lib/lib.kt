package lib

@Deprecated("gone", level = DeprecationLevel.ERROR)
fun errFun() = 1

@Deprecated("gone", level = DeprecationLevel.HIDDEN)
fun hiddenFun() = 1

@Deprecated("old")
fun warnFun() = 1

@Deprecated("gone", level = DeprecationLevel.ERROR)
class ErrClass

@Deprecated("gone", level = DeprecationLevel.HIDDEN)
class HiddenClass

@Deprecated("gone", level = DeprecationLevel.ERROR)
val errVal = 1

@Deprecated("gone", level = DeprecationLevel.ERROR)
typealias ErrAlias = String

class Holder(val x: Int) {
    @Deprecated("gone", level = DeprecationLevel.ERROR)
    constructor(s: String) : this(s.length)

    @Deprecated("gone", level = DeprecationLevel.ERROR)
    val prop = 1

    @Deprecated("gone", level = DeprecationLevel.ERROR)
    fun member() = 1
}

@Deprecated("gone", level = DeprecationLevel.ERROR)
class Outer {
    class Nested
}

@Deprecated("gone", level = DeprecationLevel.HIDDEN)
typealias HiddenAlias = String

class Box {
    @Deprecated("gone", level = DeprecationLevel.HIDDEN)
    class HiddenNested
}

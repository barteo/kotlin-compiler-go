package base

abstract class Base {
    protected val label get() = "label"
    protected var count = 1
    var guarded = "guarded"
        protected set
}

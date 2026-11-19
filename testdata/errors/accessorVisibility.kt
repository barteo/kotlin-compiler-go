// A getter has its property's visibility: one written otherwise is an
// error (GETTER_VISIBILITY_DIFFERS_FROM_PROPERTY_VISIBILITY). Both it and
// an open property's private setter are reported at the modifier.
abstract class My {
    abstract var x: Int
        public get
        protected set

    abstract val y: Int
        protected get // ERROR getter visibility must be the same as property visibility

    abstract internal val w: Int
        protected get // ERROR getter visibility must be the same as property visibility

    protected val v: Int
        protected get() = 1

    open var t: Int = 0
        private set // ERROR private setters are not allowed for open properties
}

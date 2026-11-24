interface My {
    private val x: Int // ERROR abstract property in an interface cannot be private
    private var y: Int // ERROR abstract property in an interface cannot be private
        private set
    private val xxx: Int
        get() = 0
    val ok: Int
    private fun bar() = 42
}

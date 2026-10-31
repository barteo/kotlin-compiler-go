object Registry {
    lateinit var block: () -> Unit
    @JvmField var count = 1
    lateinit var name: String
}

class Box {
    lateinit var s: String

    companion object {
        lateinit var label: String
        @JvmField var size = 10
        @JvmField val fixed = "fixed"
    }
}

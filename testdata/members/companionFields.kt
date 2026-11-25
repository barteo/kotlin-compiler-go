package members

class Holder {
    companion object {
        val shared: List<String> = listOf("a")
        var counter: Int = 0
        val isOn: Boolean = true
        private var hidden: Int = 0
        private val onlyOuter = "o"
        internal val inner: Map<String, Int> = emptyMap()
        var withField: Int = 1
            get() = field + 1
            set(v) { field = v * 2 }
        val lazyName: String by lazy { "x" }
        lateinit var late: String
        private lateinit var privateLate: String
        lateinit var privateSet: String
            private set
        @JvmField val jf: String = "j"
        const val C = 1

        fun bump() {
            hidden++
            privateLate = "p"
            privateSet = "s"
        }
    }

    fun read() = shared.size + counter + hidden + onlyOuter.length + inner.size + withField + lazyName.length
    fun write() {
        counter = 2
        hidden = 3
        withField = 4
    }
    inline fun readInline() = shared.size + counter
}

class Hidden {
    @Deprecated("x", level = DeprecationLevel.HIDDEN)
    val old: Int = 1
    @Deprecated("x", level = DeprecationLevel.HIDDEN)
    var oldVar: String = ""
    @Deprecated("x", level = DeprecationLevel.HIDDEN)
    var custom: Int = 0
        get() = field + 1
        set(v) { field = v }
    @Deprecated("x", level = DeprecationLevel.HIDDEN)
    fun oldFun(): Int = 1

    companion object {
        @Deprecated("x", level = DeprecationLevel.HIDDEN)
        var oldStatic: Int = 0
    }
}

@Deprecated("x", level = DeprecationLevel.HIDDEN)
val hiddenTop: Int = 1

interface WithCompanion {
    companion object {
        val shared: List<String> = listOf("a")
        var counter: Int = 0
        private var hidden: Int = 0
        var withField: Int = 1
            get() = field + 1
            set(v) { field = v * 2 }
        val lazyName: String by lazy { "x" }
        lateinit var late: String
        lateinit var privateSet: String
            private set
        @JvmStatic val st: Int = 5
        const val C = 1

        init {
            counter = 3
        }

        fun make(): Int = 1
        fun bump() {
            hidden++
            privateSet = "s"
        }
    }

    fun read() = shared.size + counter + withField
}

interface WithJvmFields {
    companion object {
        @JvmField val first: String = "f"
        @JvmField val second: String = first + "s"
    }
}

annotation class WithAnnotationCompanion {
    companion object {
        val name: String = "n"
        var count: Int = 0
    }
}

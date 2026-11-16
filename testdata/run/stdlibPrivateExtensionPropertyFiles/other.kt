private val Box.stamp: String get() = "other:" + data?.size

private var Box.note: String
    get() = "other note"
    set(value) = println("other note = $value")

fun Box.first(): String? = data
    ?.takeIf { it.isNotEmpty() }
    ?.let {
        note = "a"
        "first $stamp ${this.stamp} $note"
    }

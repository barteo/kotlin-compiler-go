import Pack.Companion.builder

class Pack(val tag: String) {
    class Builder {
        private var tag = ""
        fun tag(t: String) = apply { tag = t }
        fun build() = Pack(tag)
    }

    companion object {
        fun builder(): Builder = Builder()
    }
}

object Manager {
    fun builder(id: Int): Starter = Starter(id)

    class Starter internal constructor(private val id: Int) {
        fun pack(): Pack = builder().tag("starter $id").build()
    }
}

fun main() = println(Manager.builder(3).pack().tag)

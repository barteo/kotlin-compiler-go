package pool

abstract class Pool<S : Any> {
    protected var slots: Array<S?>? = null
        private set
    protected var count = 0
    protected var label: String = "pool"
        set(value) {
            field = value.uppercase()
        }

    protected abstract fun make(): S

    protected fun add() {
        @Suppress("UNCHECKED_CAST")
        val s = slots ?: (arrayOfNulls<Any>(4) as Array<S?>).also { slots = it }
        s[count++] = make()
    }

    protected inline fun forEachSlot(block: (S) -> Unit) {
        if (count == 0) return
        slots?.forEach { slot -> if (slot != null) block(slot) }
    }

    protected inline fun relabel(f: (String) -> String) {
        label = f(label)
        count += 0
        add()
    }

    override fun toString() = "$label:$count"
}

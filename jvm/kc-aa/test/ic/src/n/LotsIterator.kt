package n

// Its result's type, which Batch's loop reads, names Address: dirty when Address.kt changes.
operator fun Lots.iterator() = listOf(m.Address("x")).iterator()

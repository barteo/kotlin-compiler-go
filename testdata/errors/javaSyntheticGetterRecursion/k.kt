// A call mode() that the class's property mode, being inferred, gives
// way to tries the synthetic property mode of the Java getter getMode()
// first, as kotlinc does (KT-57166): the program's override of it, with
// an inferred return type, reads the property and runs into the
// recursion.

fun mode(): String = ""
fun count(): Int = 0

class Rec : Desc {
    private val mode = mode()
    override fun getMode() = mode // ERROR recursive problem
    override fun getCount() = 1
}

class Other : Desc {
    private val count = count()
    override fun getMode() = ""
    override fun getCount() = count // ERROR recursive problem
}

class Typed : Desc {
    private val mode = mode()
    override fun getMode(): String = mode
    override fun getCount() = 1
}

class NotReading : Desc {
    private val mode = mode()
    override fun getMode() = "x"
    override fun getCount() = 1
}

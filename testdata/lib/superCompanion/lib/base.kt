package base

// A class whose companion's functions its subclasses call unqualified,
// as Android fragments' newBundle().
open class Base {
    companion object {
        fun newBundle(data: String): MutableMap<String, String> = mutableMapOf("data" to data)
    }
}

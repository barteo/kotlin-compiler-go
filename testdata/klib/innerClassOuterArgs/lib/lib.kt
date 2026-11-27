package q

abstract class Call<S : CharSequence>(val name: S) {
    inner class Args : ArrayList<String?>()

    inner class Names : ArrayList<S>()

    inner class Tagged<T>(val value: T) {
        fun describe() = "$name:$value"
    }

    val arguments: Args = Args()
    val names: Names = Names()

    fun tagged(n: Int): Tagged<Int> = Tagged(n)
}

class NamedCall(name: String) : Call<String>(name)

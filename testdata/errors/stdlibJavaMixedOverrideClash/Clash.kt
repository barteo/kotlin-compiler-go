abstract class KA : A() {
    override fun contains(x: String) = false // ERROR accidentally overrides both 'contains'
}

abstract class KOpen : OpenCharAt() {
    override fun get(index: Int) = 'A' // ERROR accidentally overrides both 'get'
}

class KFinal : FinalCharAt() {
    override fun get(index: Int) = 'A'
}

class KProxy : FinalCharAt(), Proxy {
    override fun get(index: Int) = 'A'
}

abstract class KOnly : OnlyCharAt() {
    override fun get(index: Int) = 'A'
}

abstract class KX : X() {
    override fun removeAt(index: Int) = "" // ERROR accidentally overrides both 'removeAt'
}

abstract class KY : X()

fun use(a: A, ka: KA, x: X) {
    a.contains("")
    ka.contains("")
    x.removeAt(0)
}

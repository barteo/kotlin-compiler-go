import q.Req
import q.nameOf

class Scope {
    fun <T> record(block: Scope.() -> T): T = block()
}

class R(val n: String) : Req<String> {
    override fun clone(): Req<String> = R("$n copy")

    override fun name() = n
}

class Test {
    fun run() {
        with(R("a")) {
            println(nameOf(Scope().record { clone() }))
            println(Scope().record { name() })
        }
    }
}

class Copyable(val v: Int) : Cloneable {
    public override fun clone(): Copyable = super.clone() as Copyable
}

fun main() {
    Test().run()
    println(Copyable(4).clone().v)
}

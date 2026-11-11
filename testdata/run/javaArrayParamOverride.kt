import java.lang.reflect.InvocationHandler
import java.lang.reflect.Method
import java.lang.reflect.Proxy

interface Greeter {
    fun greet(name: String, times: Int): String
}

class Handler : InvocationHandler {
    override fun invoke(proxy: Any?, method: Method?, args: Array<Any?>?): Any? {
        val a = args!!
        return method!!.name + " " + a[0] + " x" + a[1]
    }
}

class OutHandler : InvocationHandler {
    override fun invoke(proxy: Any?, method: Method?, args: Array<out Any?>?): Any? = "out " + args!!.size
}

fun main() {
    val loader = Greeter::class.java.classLoader
    val types = arrayOf<Class<*>>(Greeter::class.java)
    val g = Proxy.newProxyInstance(loader, types, Handler()) as Greeter
    println(g.greet("kc", 2))
    val h = Proxy.newProxyInstance(loader, types, OutHandler()) as Greeter
    println(h.greet("kc", 3))
}

// kotlinc: -Xmulti-platform -Xcommon-sources=expectActual/common.kt,expectActual/common/shared.kt
package platform

fun main() {
    println(greet("kc"))
    println(answer)
    println("hi".shout())
    println(firstOf("a", "b"))
    val c = Counter(10)
    c.next()
    println(c.next())
    println(c.current)
    println(Clock.now())
    println(Lock().withLock { "locked" })
    println(shared() + " " + twice(4))
    // Platform.jvm.kt's facade, as kotlinc names it.
    println(Class.forName("platform.Platform_jvmKt").simpleName)
}

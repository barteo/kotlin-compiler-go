// A local class in a builder's lambda, in a local class, that calls a
// member of the class around: its instance is captured.
class Buildee<T> { var v: Any? = null; fun consume(t: T) { v = t } }
fun <T> build(f: Buildee<T>.() -> Unit): Buildee<T> = Buildee<T>().apply(f)

class Klass {
    val value55 = run {
        class LocalA {
            val valueA = build a@{
                class LocalB {
                    val valueB = build b@{
                        class LocalC {
                            fun baz() {
                                this@a.consume(bar())
                                this@b.consume(bar())
                            }
                        }
                        LocalC().baz()
                    }
                    private fun bar() = foo()
                }
                println(LocalB().valueB.v)
            }
            private fun foo() = "foo"
        }
        LocalA().valueA.v
    }
}
fun main() { println(Klass().value55) }

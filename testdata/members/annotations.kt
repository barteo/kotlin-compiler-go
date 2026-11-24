// The $annotations methods of annotated properties, which have the
// property's visibility (TestMembers).
package a

annotation class Ann

open class A(@property:Ann protected val c: Int, @property:Ann private val d: Int, @property:Ann val e: Int) {
    @Ann protected val p: Int = 1
    @Ann protected open val o: Int = 1
    @Ann internal val i: Int = 2
    @Ann private val q: Int = 3
    @Ann val pub: Int = 4
    @Ann protected val String.ext: Int get() = 1
    @Ann private val String.privateExt: Int get() = 1
    @Ann protected var privateSet: Int = 1
        private set
    @Ann protected lateinit var late: String
    @Ann private val privateGetter: Int get() = 3
    @Ann private var privateAccessors: Int = 1
        get() = field + 1
        set(v) { field = v }
    @Ann private val privateLazy by lazy { 1 }
    @Ann protected val protectedLazy by lazy { 1 }
    fun use() = q + d + "".privateExt + privateGetter + privateAccessors + privateLazy

    companion object {
        @Ann protected val inCompanion: Int = 1
        @Ann @JvmStatic protected val static: Int = 1
        @Ann protected const val CONST: Int = 1
        @Ann @JvmField protected val field: Int = 1
    }
}

class B : A(1, 2, 3) {
    @Ann override val o: Int = 2
}

abstract class C {
    @Ann protected abstract val a: Int
}

object O {
    @Ann private val x = 1
    @Ann val y = 2
}

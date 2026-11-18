// Where @JvmField can't make a property a public field.
abstract class C {
    @JvmField val a: String by lazy { "A" } // ERROR JvmField cannot be applied to a delegated property
    @JvmField open val b: Int = 3 // ERROR JvmField cannot be applied to a property that overrides some other property or can be overridden
    @JvmField abstract val c: Int // ERROR this annotation is not applicable to target 'member property without backing field'
    @JvmField // ERROR JvmField cannot be applied to a property with a custom accessor
    val explicitDefaultGetter: String = ""
        get
    @JvmField // ERROR JvmField has no effect on a private property
    private val p = 3
    @JvmField val ok = 3
}

class G {
    @JvmField // ERROR JvmField cannot be applied to a lateinit property
    lateinit var lateInit: String
}

interface K {
    companion object {
        @JvmField // ERROR could be applied only if all interface companion properties are 'public final val'
        val c = 3
        val x = 3
    }
}

interface KOk {
    companion object {
        @JvmField val c = 3
    }
}

object IObject {
    @JvmField // ERROR JvmField cannot be applied to a property of an inline class type
    val u = 42u
}

// @JvmName can't rename an override, a member that can be overridden, or
// a local function (INAPPLICABLE_JVM_NAME): overrides would keep the JVM
// name they override.
fun foo1() {
    @JvmName("a") // ERROR '@JvmName' annotation is not applicable to this declaration
    fun foo() {}
}

abstract class AB {
    @JvmName("AB_absFun1") // ERROR '@JvmName' annotation is not applicable to this declaration
    abstract fun absFun1()

    @JvmName("AB_openFun") // ERROR '@JvmName' annotation is not applicable to this declaration
    open fun openFun() {}

    @JvmName("AB_protectedFinalFun")
    protected fun protectedFinalFun() {}

    @JvmName("AB_privateFinalFun")
    private fun privateFinalFun() {}
}

class D : AB() {
    override fun absFun1() {}

    @JvmName("D_openFun") // ERROR '@JvmName' annotation is not applicable to this declaration
    final override fun openFun() {}

    @JvmName("D_finalFun")
    fun finalFun() {}
}

interface Intf {
    @get:JvmName("getBar") // ERROR '@JvmName' annotation is not applicable to this declaration
    var foo: Int

    @Suppress("INAPPLICABLE_JVM_NAME")
    @JvmName("renamed")
    fun suppressed()
}

// kotlinc matches @Suppress names whatever their case, on the enclosing
// declarations too.
@Suppress("inapplicable_jvm_name")
interface Factory {
    @get:JvmName("supportsMultilevelIntrospection")
    val supportsMultilevelIntrospection: Boolean
        get() = false
}

open class Some {
    companion object {
        // An object's open member can't be overridden.
        @get:JvmName("getInstance")
        open val INSTANCE: String
            get() = "Omega"
    }
}

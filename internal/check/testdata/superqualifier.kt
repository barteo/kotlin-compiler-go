// The warnings kotlinc 2.4.21 reports of the super qualifiers below, as
// TestWarningsSuperQualifier reads them: each line ends with a comment naming
// the warnings it reports, if any.
interface B<T> {
    fun f() = true
    val p: Int get() = 1
}
interface D<K, V> { fun g() = 1 }
open class A(b: Boolean)
open class G<T> { open fun h() {} }
typealias BI = B<Int>

class C : B<Int>, D<String, Int>, G<Long>() {
    fun t1() = super<B>.f()
    fun t2() = super<B<Int>>.f()
    fun t3() = super<D<String, Int>>.g()
    fun t4() = super<B<Int>>.p
    override fun h() { super<G<Long>>.h() }
    fun t5() = super<BI>.f()
    fun t6() = super<B<Int>>@C.f()
    @Suppress("TYPE_ARGUMENTS_REDUNDANT_IN_SUPER_QUALIFIER")
    fun t7() = super<B<Int>>.f()
    inner class Inner : A(super<B>.f())
    inner class Inner2 : A(super<B<Int>>.f()) // TYPE_ARGUMENTS_REDUNDANT_IN_SUPER_QUALIFIER
    inner class Inner3 { fun t() = super<B<Int>>@C.f() } // TYPE_ARGUMENTS_REDUNDANT_IN_SUPER_QUALIFIER
    fun test() {
        class LocalClass2 : A(super<B<Int>>.f()) // TYPE_ARGUMENTS_REDUNDANT_IN_SUPER_QUALIFIER
    }
}

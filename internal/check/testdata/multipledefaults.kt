// The MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES_DEPRECATION_WARNING
// warnings kotlinc 2.4.21 reports of the overrides below, whose default
// values come from more than one supertype but at most one direct one,
// as TestWarningsMultipleDefaults reads them: each line ends with a
// comment naming the warnings it reports, if any.
interface Base1<K> {
    fun child(props: Int = 20)
}
interface Override1<J> : Base1<J>
open class Keker1<P> {
    open fun child(props: Int = 10) {}
}
class Impl1<P> : Keker1<P>(), Override1<P> {
    override fun child(props: Int) {} // MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES_DEPRECATION_WARNING
}

interface Override2<J> : Base1<J> {
    override fun child(props: Int) {}
}
class Impl2<P> : Keker1<P>(), Override2<P> {
    override fun child(props: Int) {} // MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES_DEPRECATION_WARNING
}

interface Foo {
    fun foo(param: Int = 1, other: String = "")
}
interface Bar {
    fun foo(param: Int = 2, other: String)
}
interface Baz1 : Bar
class Baz : Baz1, Foo {
    override fun foo(param: Int, other: String) {} // MULTIPLE_DEFAULTS_INHERITED_FROM_SUPERTYPES_DEPRECATION_WARNING
}

interface Single {
    fun s(x: Int = 1)
}
interface Single2 : Single
class OneDefault : Single2 {
    override fun s(x: Int) {}
}

interface A
interface WithDefault {
    fun w(x: Int = 1)
}
interface NoDefault {
    fun w(x: Int)
}
class Mixed : WithDefault, NoDefault {
    override fun w(x: Int) {}
}

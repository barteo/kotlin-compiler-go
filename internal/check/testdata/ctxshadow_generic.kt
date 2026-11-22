// The warnings kotlinc 2.4.21 reports of the generic contextual overloads below, as
// TestWarningsContextualShadowingGeneric reads them: each line ends with a comment naming
// the warnings it reports, if any.
interface Sup
interface Sub : Sup
interface Other
interface G<T>
interface I<A, B>
interface R

context(ctx: T?) fun <T : Any> f8(): R = null!!
@JvmName("f8b") context(ctx: T) fun <T : Any> f8(): R = null!! // CONTEXTUAL_OVERLOAD_SHADOWED

context(ctx: T) fun <T> f9(): R = null!!
@JvmName("f9b") context(ctx: T & Any) fun <T> f9(): R = null!! // CONTEXTUAL_OVERLOAD_SHADOWED

context(ctx: T) fun <T : Sup> f11(): R = null!!
@JvmName("f11b") context(ctx: T) fun <T : Sub> f11(): R = null!! // CONTEXTUAL_OVERLOAD_SHADOWED

context(ctx: Sup) fun <T> f12(): R = null!!
@JvmName("f12b") context(ctx: T) fun <T : Sub> f12(): R = null!! // CONTEXTUAL_OVERLOAD_SHADOWED

context(ctx: T) fun <T : Sup> f13(): R = null!!
@JvmName("f13b") context(ctx: Sub) fun <T> f13(): R = null!! // CONTEXTUAL_OVERLOAD_SHADOWED

context(ctx: T) fun <T : Sup> f15(): R = null!!
@JvmName("f15b") context(ctx: T) fun <T : Other> f15(): R = null!!

context(ctx: G<T>) fun <T> f16(): R = null!!
@JvmName("f16b") context(ctx: G<T>) fun <T : Any> f16(): R = null!! // CONTEXTUAL_OVERLOAD_SHADOWED

context(ctx: G<T>) fun <T> f17(x: T): R = null!!
@JvmName("f17b") context(ctx: G<T>, o: Other) fun <T> f17(x: T): R = null!! // CONTEXTUAL_OVERLOAD_SHADOWED

context(c: I<A, B>) fun <A, B> f18(left: A, right: B) {}
@JvmName("f18b") context(c: I<A, B>, _: Map<A, B>) fun <A, B> f18(left: A, right: B) {} // CONTEXTUAL_OVERLOAD_SHADOWED

context(c: Sup) fun <T> f19(x: T) {}
@JvmName("f19b") context(c: Sub) fun <T> f19(x: List<T>) {}

context(c: Sup) fun <T> T.f20() {}
@JvmName("f20b") context(c: Sub) fun <T> T.f20() {} // CONTEXTUAL_OVERLOAD_SHADOWED

context(c: Sup) fun <T> f21(x: T) {}
@JvmName("f21b") context(c: Sub) fun <T, U> f21(x: T) {}

context(ctx: T?) val <T : Any> p8: R get() = null!!
@get:JvmName("p8b") context(ctx: T) val <T : Any> p8: R get() = null!! // CONTEXTUAL_OVERLOAD_SHADOWED

context(ctx: T) val <T> p9: R get() = null!!
@get:JvmName("p9b") context(ctx: T & Any) val <T> p9: R get() = null!! // CONTEXTUAL_OVERLOAD_SHADOWED


context(_: T) val <T> p12: Int get() = 0
@get:JvmName("p12b") context(_: U) val <U : Any> p12: String get() = "" // CONTEXTUAL_OVERLOAD_SHADOWED

context(_: List<T>) val <T> p13: Int get() = 0
@get:JvmName("p13b") context(_: List<U>) val <U : Any> p13: String get() = "" // CONTEXTUAL_OVERLOAD_SHADOWED

class C<K> {
    context(ctx: T) fun <T : Sub> m2(): R = null!!
    @JvmName("m2b") context(ctx: T) fun <T : Sup> m2(): R = null!!
}

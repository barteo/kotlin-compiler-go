// A Java annotation without @Repeatable doesn't repeat, even without a
// @Target; one with it does, but not together with its container,
// unless it's source-retained. Nor a Kotlin one.

@java.lang.annotation.Repeatable(KR.Container::class)
@Retention(AnnotationRetention.RUNTIME)
annotation class KR {
    annotation class Container(val value: Array<KR>)
}

@JR @JR fun ok1() {}
@JR @JR.Container fun ok2() {}
@JS @JS.Container @JS fun ok3() {}
@Once fun ok4() {}
@KR @KR.Container([]) fun ok5() {}

@Once
@Once // ERROR not repeatable
fun twice() {}

@JR
@JR.Container
@JR // ERROR used with its container
fun withContainer() {}

@KR
@KR // ERROR used with its container
@KR.Container([KR()])
fun kotlinWithContainer() {}

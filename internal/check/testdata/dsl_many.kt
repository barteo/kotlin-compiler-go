// The warnings kotlinc 2.4.21 reports of the annotated function types below, as
// TestWarningsDslMarkerPropagation reads them: each line ends with a comment naming
// the warnings it reports, if any.
@DslMarker @Target(AnnotationTarget.CLASS, AnnotationTarget.TYPE, AnnotationTarget.TYPEALIAS)
annotation class Marker
@DslMarker @Target(AnnotationTarget.CLASS, AnnotationTarget.TYPE)
annotation class Marker2
@Target(AnnotationTarget.TYPE)
annotation class Plain

class A
class B
class C

fun report1(foo: @Marker context(A) B.() -> Unit) { } // DSL_MARKER_PROPAGATES_TO_MANY
fun report2(foo: @Marker context(A, B) () -> Unit) { } // DSL_MARKER_PROPAGATES_TO_MANY
fun report3(foo: @Marker context(A, B) C.() -> Unit) { } // DSL_MARKER_PROPAGATES_TO_MANY
fun report4(foo: @Marker (context(A) B.() -> Unit)) { } // DSL_MARKER_PROPAGATES_TO_MANY
fun report5(foo: @Plain @Marker @Marker2 context(A) B.() -> Unit) { } // DSL_MARKER_PROPAGATES_TO_MANY
fun report6(foo: (@Marker context(A) B.() -> Unit)?) { } // DSL_MARKER_PROPAGATES_TO_MANY
fun report7(): @Marker context(A) B.() -> Unit = {} // DSL_MARKER_PROPAGATES_TO_MANY
val report8: @Marker context(A) B.() -> Unit = {} // DSL_MARKER_PROPAGATES_TO_MANY
fun report9(foo: List<@Marker context(A) B.() -> Unit>) { } // DSL_MARKER_PROPAGATES_TO_MANY
fun report10(foo: @Marker suspend context(A) B.() -> Unit) { } // DSL_MARKER_PROPAGATES_TO_MANY

fun inside1(foo: context(@Marker A) B.() -> Unit) { }
fun inside2(foo: context(@Marker A, B) () -> Unit) { }
fun inside3(foo: context(A, B) (@Marker C).() -> Unit) { }

fun ok1(foo: @Marker () -> Unit) { }
fun ok2(foo: @Marker context(A) () -> Unit) { }
fun ok3(foo: @Marker B.() -> Unit) { }
fun ok4(foo: @Plain context(A) B.() -> Unit) { }

@Marker
typealias TA = context(A) B.() -> Unit
fun alias1(foo: TA) { }
typealias TB = @Marker context(A) B.() -> Unit // DSL_MARKER_PROPAGATES_TO_MANY
fun alias2(foo: TB) { }

fun nothing1(foo: context(A) B.() -> Unit) { }
@Suppress("DSL_MARKER_PROPAGATES_TO_MANY")
fun suppressed(foo: @Marker context(A) B.() -> Unit) { }

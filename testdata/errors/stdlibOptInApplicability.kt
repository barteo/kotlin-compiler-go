// @SubclassOptInRequired is for classes others can subclass, and opt-in
// annotations can only be used as annotations.
@RequiresOptIn
annotation class ApiMarker

@SubclassOptInRequired(ApiMarker::class) // ERROR '@SubclassOptInRequired' is not applicable to a final class
class Klass

@SubclassOptInRequired(ApiMarker::class) // ERROR '@SubclassOptInRequired' is not applicable to a sealed class
sealed class SealedKlass

@SubclassOptInRequired(ApiMarker::class)
open class Open

@SubclassOptInRequired(ApiMarker::class)
interface Iface

fun foo() {
    @SubclassOptInRequired(ApiMarker::class) // ERROR '@SubclassOptInRequired' is not applicable to a local class
    open class LocalOpenKlass
}

fun f1(e: RequiresOptIn) {} // ERROR this class can only be used as an annotation
typealias Marker0 = ApiMarker // ERROR opt-in requirement marker 'ApiMarker' can only be used as an annotation
fun f2(m: List<ApiMarker>?) {} // ERROR opt-in requirement marker 'ApiMarker' can only be used as an annotation

@OptIn(ApiMarker::class)
fun ok() {}

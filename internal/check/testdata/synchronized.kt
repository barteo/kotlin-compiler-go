// SYNCHRONIZED_ON_INLINE: @Synchronized on an inline function, an inline
// accessor or an inline property's accessor, whose body is copied into
// its callers without the lock.
@Synchronized // SYNCHRONIZED_ON_INLINE
inline fun foo(f: () -> Unit): Unit = f()

var bar: String
    @Synchronized // SYNCHRONIZED_ON_INLINE
    inline get() = ""
    @Synchronized // SYNCHRONIZED_ON_INLINE
    inline set(value) {}

inline var baz: String
    @Synchronized // SYNCHRONIZED_ON_INLINE
    get() = ""
    @Synchronized // SYNCHRONIZED_ON_INLINE
    set(value) {}

@get:Synchronized // SYNCHRONIZED_ON_INLINE
inline val qux: String get() = ""

@set:Synchronized @get:Synchronized // SYNCHRONIZED_ON_INLINE
var quux: String
    inline get() = ""
    set(value) {}

class C {
    @Synchronized // SYNCHRONIZED_ON_INLINE
    inline fun m(f: () -> Unit) = f()
    @Synchronized
    fun n() {}
    @Suppress("SYNCHRONIZED_ON_INLINE")
    @Synchronized
    inline fun o(f: () -> Unit) = f()
}

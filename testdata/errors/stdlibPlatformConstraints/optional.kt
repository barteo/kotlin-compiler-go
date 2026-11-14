import java.util.Optional

// K2's predefined enhancements of java.util.Optional: of takes a
// T & Any, ofNullable a T?, and both return an Optional<T & Any>.

fun take(o: Optional<String>) {}

fun use(x: String?, y: String) {
    take(Optional.ofNullable(x))
    take(Optional.of(y))
    val s: String = Optional.of(y).get()
    Optional.of(x) // ERROR cannot infer type for type parameter 'T'
    Optional.ofNullable(x).ifPresent { it.length }
}

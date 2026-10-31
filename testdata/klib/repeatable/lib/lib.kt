package tags

// A repeatable annotation, which kotlinc gives a container class,
// Tag.Container, as Compose's @Preview.
@Repeatable
@Retention(AnnotationRetention.RUNTIME)
annotation class Tag(val name: String)

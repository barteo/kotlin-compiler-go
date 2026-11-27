// An unannotated use-site target on a constructor property goes to the
// parameter, or else to the property, or else to the field; @delegate:
// needs a delegated property.

@Target(AnnotationTarget.FIELD, AnnotationTarget.PROPERTY)
annotation class FieldOrProp

@Target(AnnotationTarget.FIELD)
annotation class FieldOnly

annotation class Any3

class B(
    @property:FieldOrProp @FieldOrProp val y: Int, // ERROR this annotation is not repeatable
    @field:FieldOnly @FieldOnly val z: Int, // ERROR this annotation is not repeatable
    @field:FieldOrProp @FieldOrProp val w: Int,
    @param:Any3 @property:Any3 val q: Int,
    @delegate:FieldOnly val d: Int, // ERROR '@delegate:' annotations can only be applied to delegated properties
) {
    @field:FieldOrProp @delegate:FieldOrProp // ERROR '@delegate:' annotations can only be applied to delegated properties
    val v: Int = 42

    @delegate:FieldOnly
    val u: Int by lazy { 42 }

    @delegate:FieldOnly // ERROR '@delegate:' annotations can only be applied to delegated properties
    lateinit var l: String
}

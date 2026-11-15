// POSITIONED_VALUE_ARGUMENT_FOR_JAVA_ANNOTATION: of a Java annotation,
// only the value element takes positional arguments.

@A("v", a = 1) fun ok1() {}
@B("x", "y", "z") fun ok2() {}
@A("v", 1) fun bad() {} // ERROR only named arguments

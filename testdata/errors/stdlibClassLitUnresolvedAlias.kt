// A class literal of a typealias of an unresolved class is no constant.

import kotlin.reflect.KClass

public annotation class Throws(vararg val exceptionClasses: KClass<out Throwable>)

typealias UEAlias = UE // ERROR unresolved or unsupported type: UE

@Throws(UEAlias::class) // ERROR annotation argument must be a compile-time constant
fun throwsTypealiasToUnresolved() {}

/* GENERATED_FIR_TAGS: annotationDeclaration, classReference, functionDeclaration, outProjection, primaryConstructor,
propertyDeclaration, typeAliasDeclaration, vararg */

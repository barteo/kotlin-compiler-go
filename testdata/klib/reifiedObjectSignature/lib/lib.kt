package lib

import java.lang.reflect.ParameterizedType

abstract class TypeRef<T> {
    val type = (javaClass.genericSuperclass as ParameterizedType).actualTypeArguments[0]
}

inline fun <reified T> typeRef(): TypeRef<T> = object : TypeRef<T>() {}

inline fun <reified T> describe(): String = typeRef<T>().type.toString()

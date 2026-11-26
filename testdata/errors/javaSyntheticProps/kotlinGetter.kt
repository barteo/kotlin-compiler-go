abstract class KotlinGetterImpl : KotlinGetterSide {
    override fun getSome(): String = ""
}

fun kotlinGetter(k: KotlinGetterImpl) {
    k.some = ""
    k.some = 1 // ERROR type mismatch
}

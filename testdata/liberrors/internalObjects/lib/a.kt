package a

public class PublicClass {
    internal companion object {
        fun f() {}
    }

    internal object Obj

    object PublicObj
}

public class WithCompanion {
    companion object
}

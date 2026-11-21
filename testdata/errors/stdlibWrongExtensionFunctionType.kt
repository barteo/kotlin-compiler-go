@Target(AnnotationTarget.TYPE)
annotation class Ann

interface A : (String) -> Unit
typealias AliasedEFT = ExtensionFunctionType

interface Some {
    fun f1(): String.() -> Int
    fun f2(): @ExtensionFunctionType() (String.() -> Int)
    fun f3(): @ExtensionFunctionType Function1<String, Int>
    fun f4(): @ExtensionFunctionType @Ann (String) -> Int
    fun f5(): @AliasedEFT Function1<String, Int>

    fun g1(): @ExtensionFunctionType () -> Int // ERROR '@ExtensionFunctionType' is only allowed on function types with at least one parameter
    fun g2(x: @ExtensionFunctionType Function0<Int>) // ERROR '@ExtensionFunctionType' is only allowed on function types with at least one parameter
    fun g3(x: @ExtensionFunctionType Int) // ERROR '@ExtensionFunctionType' is only allowed on function types with at least one parameter
    fun g4(x: @AliasedEFT A) // ERROR '@ExtensionFunctionType' is only allowed on function types with at least one parameter
}

fun use(s: Some) = "".(s.f5())()

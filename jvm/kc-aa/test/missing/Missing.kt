// Missing is a processor that writes, to resources/missing.txt, the
// declarations of m.Lib, a library class whose members name m.Missing, a
// class that isn't on the class path: in the order of their JVM names, as
// KSP orders a binary class's members (Dagger lists a constructor
// parameter's class's so), with each function's JVM name. KSP asks for
// that whether the types of a function's parameters and result are value
// classes, which a class kc doesn't have is not.
import com.google.devtools.ksp.KspExperimental
import com.google.devtools.ksp.processing.Dependencies
import com.google.devtools.ksp.processing.Resolver
import com.google.devtools.ksp.processing.SymbolProcessor
import com.google.devtools.ksp.processing.SymbolProcessorEnvironment
import com.google.devtools.ksp.processing.SymbolProcessorProvider
import com.google.devtools.ksp.symbol.KSAnnotated
import com.google.devtools.ksp.symbol.KSFunctionDeclaration
import com.google.devtools.ksp.symbol.KSPropertyDeclaration

class MissingProvider : SymbolProcessorProvider {
    override fun create(environment: SymbolProcessorEnvironment) = object : SymbolProcessor {
        private var done = false

        @OptIn(KspExperimental::class)
        override fun process(resolver: Resolver): List<KSAnnotated> {
            if (done) return emptyList()
            done = true
            val lib = resolver.getClassDeclarationByName(resolver.getKSNameFromString("m.Lib"))!!
            val lines = resolver.getDeclarationsInSourceOrder(lib).map { d ->
                when (d) {
                    is KSFunctionDeclaration -> "fun ${d.simpleName.asString()}: ${resolver.getJvmName(d)}"
                    is KSPropertyDeclaration ->
                        "val ${d.simpleName.asString()}: ${d.getter?.let { resolver.getJvmName(it) }}"
                    else -> "${d.javaClass.simpleName} ${d.simpleName.asString()}"
                }
            }.toList()
            environment.codeGenerator.createNewFile(Dependencies(false), "", "missing", "txt")
                .use { it.write(lines.joinToString("\n", postfix = "\n").toByteArray()) }
            return emptyList()
        }
    }
}

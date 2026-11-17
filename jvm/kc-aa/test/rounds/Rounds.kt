// Rounds is a processor that generates the Kotlin class r.Made in its first
// round and writes, to resources/rounds.txt in its second, what the class
// literals in the annotations of r.Uses, a Java source class, named in each
// round: one that names r.Made resolves once the class exists.
import com.google.devtools.ksp.processing.Dependencies
import com.google.devtools.ksp.processing.Resolver
import com.google.devtools.ksp.processing.SymbolProcessor
import com.google.devtools.ksp.processing.SymbolProcessorEnvironment
import com.google.devtools.ksp.processing.SymbolProcessorProvider
import com.google.devtools.ksp.symbol.KSAnnotated
import com.google.devtools.ksp.symbol.KSType

class RoundsProvider : SymbolProcessorProvider {
    override fun create(environment: SymbolProcessorEnvironment) = object : SymbolProcessor {
        private val lines = ArrayList<String>()
        private var round = 0

        private fun describe(v: Any?): String = when (v) {
            is KSType -> if (v.isError) "error type" else v.declaration.qualifiedName!!.asString()
            is List<*> -> v.joinToString(", ", "[", "]") { describe(it) }
            else -> v.toString()
        }

        override fun process(resolver: Resolver): List<KSAnnotated> {
            round++
            val uses = resolver.getClassDeclarationByName(resolver.getKSNameFromString("r.Uses"))!!
            for (a in uses.annotations) {
                for (arg in a.arguments) lines += "round $round: ${arg.name?.asString()} = ${describe(arg.value)}"
            }
            when (round) {
                1 -> environment.codeGenerator.createNewFile(Dependencies(false), "r", "Made").use {
                    it.write("package r\n\nclass Made {\n    class Part\n}\n".toByteArray())
                }
                2 -> environment.codeGenerator.createNewFile(Dependencies(false), "", "rounds", "txt").use {
                    it.write(lines.joinToString("\n", postfix = "\n").toByteArray())
                }
            }
            return emptyList()
        }
    }
}

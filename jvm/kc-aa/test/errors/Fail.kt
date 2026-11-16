// Fail is a processor that reports an error for each class annotated with
// e.Bad, so the run ends as a processor's failure does: KSP then checks
// the sources, and the errors it reports are the processor's only.
import com.google.devtools.ksp.processing.Resolver
import com.google.devtools.ksp.processing.SymbolProcessor
import com.google.devtools.ksp.processing.SymbolProcessorEnvironment
import com.google.devtools.ksp.processing.SymbolProcessorProvider
import com.google.devtools.ksp.symbol.KSAnnotated
import com.google.devtools.ksp.symbol.KSClassDeclaration

class FailProvider : SymbolProcessorProvider {
    override fun create(environment: SymbolProcessorEnvironment) = object : SymbolProcessor {
        override fun process(resolver: Resolver): List<KSAnnotated> {
            resolver.getSymbolsWithAnnotation("e.Bad").filterIsInstance<KSClassDeclaration>()
                .map { it.qualifiedName!!.asString() }.sorted()
                .forEach { environment.logger.error("bad class $it") }
            return emptyList()
        }
    }
}

// Rounds is a processor that generates the Kotlin class r.Made in its first
// round and writes, to resources/rounds.txt, what the class literals in the
// annotations of r.Uses, a Java source class, named in each of the first two
// rounds: one that names r.Made resolves once the class exists.
//
// Its first round also generates the Java classes g.First and h.Third and
// the Kotlin class k.Other, each in a package no other source is in, and its
// second the Java class r.Second, whose annotation names them through a
// single-type import, an on-demand import and a qualified name. Its third
// round writes what those class literals name, and the annotations of the
// class the first one names.
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

        private fun literals(resolver: Resolver, cls: String) {
            val c = resolver.getClassDeclarationByName(resolver.getKSNameFromString(cls))!!
            for (a in c.annotations) {
                for (arg in a.arguments) lines += "round $round: $cls ${arg.name?.asString()} = ${describe(arg.value)}"
            }
        }

        private fun write(pkg: String, name: String, ext: String, text: String) =
            environment.codeGenerator.createNewFile(Dependencies(false), pkg, name, ext).use { it.write(text.toByteArray()) }

        override fun process(resolver: Resolver): List<KSAnnotated> {
            round++
            if (round <= 2) literals(resolver, "r.Uses")
            when (round) {
                1 -> {
                    write("r", "Made", "kt", "package r\n\nclass Made {\n    class Part\n}\n")
                    write("k", "Other", "kt", "package k\n\nclass Other\n")
                    write("h", "Third", "java", "package h;\n\npublic class Third {\n}\n")
                    write(
                        "g", "First", "java",
                        "package g;\n\n@r.Ref({})\npublic class First {\n    public static class Part {\n    }\n}\n",
                    )
                }
                2 -> write(
                    "r", "Second", "java",
                    "package r;\n\nimport g.First;\nimport h.*;\nimport k.Other;\n\n" +
                        "@Ref({First.class, First.Part.class, g.First.class, Third.class, Other.class, Kept.class, Made.class})\n" +
                        "public class Second {\n}\n",
                )
                3 -> {
                    literals(resolver, "r.Second")
                    val second = resolver.getClassDeclarationByName(resolver.getKSNameFromString("r.Second"))!!
                    val first = (second.annotations.single().arguments.single().value as List<*>).first() as KSType
                    val annotations = first.declaration.annotations.joinToString { "@" + it.shortName.asString() }
                    lines += "round 3: ${describe(first)} has $annotations"
                    write("", "rounds", "txt", lines.joinToString("\n", postfix = "\n"))
                }
            }
            return emptyList()
        }
    }
}

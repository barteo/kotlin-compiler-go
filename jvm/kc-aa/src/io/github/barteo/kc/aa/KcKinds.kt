package io.github.barteo.kc.aa

import com.intellij.lang.ASTNode
import com.intellij.psi.TokenType
import com.intellij.psi.tree.IElementType
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.kdoc.lexer.KDocTokens
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.psi.stubs.elements.KtStubElementTypes
import java.lang.reflect.Modifier
import java.util.IdentityHashMap

/**
 * kc's psi kind numbers (`psi/kinds.txt`, bundled in the jar) for
 * kotlin-compiler's element types, named as `tools/psidump` names them: by
 * the static fields of [KtNodeTypes], [KtStubElementTypes], [KtTokens],
 * [KDocTokens] and [TokenType] that hold them, preferring a name kc has.
 * The PSI join checks a file's tree against kc's with [hash].
 */
internal object KcKinds {
    private val numbers: Map<String, Int> = run {
        val text = KcKinds::class.java.getResourceAsStream("kinds.txt")?.bufferedReader()?.readText()
            ?: error("kc: no kinds.txt in kc-aa.jar")
        text.trim().split(Regex("\\s+")).withIndex().associate { (i, k) -> k to i + 1 }
    }
    private val aliases = mapOf("EXCLEQEQEQ" to "EXCLEQEQ")
    private val file = numbers.getValue("FILE")

    private val byType: IdentityHashMap<IElementType, Int> = IdentityHashMap<IElementType, String>().let { names ->
        for (c in listOf(KtNodeTypes::class.java, KtStubElementTypes::class.java, KtTokens::class.java, KDocTokens::class.java, TokenType::class.java)) {
            for (f in c.fields) {
                if (!Modifier.isStatic(f.modifiers) || !IElementType::class.java.isAssignableFrom(f.type)) continue
                val t = f.get(null) as? IElementType ?: continue
                val name = aliases[f.name] ?: f.name
                val prev = names[t]
                if (prev == null || prev !in numbers && name in numbers) names[t] = name
            }
        }
        IdentityHashMap<IElementType, Int>().also { m -> names.forEach { (t, n) -> m[t] = numbers[n] ?: 0 } }
    }

    /**
     * The hash of the kinds of [nodes], a tree in preorder from index 1 as
     * [KcRound.preorder] lists it, as Go's `psi.Snapshot.KindSequenceHash`
     * computes it: FNV-1a, a step per node. An element type kc has no kind
     * for counts as 0, which no kc node has.
     */
    fun hash(nodes: Array<ASTNode>): Long {
        var h = -3750763034362895579L // 14695981039346656037
        for (i in 1 until nodes.size) {
            val k = if (i == 1) file else byType[nodes[i].elementType] ?: 0
            h = (h xor k.toLong()) * 1099511628211L
        }
        return h
    }
}

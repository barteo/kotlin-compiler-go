package io.github.barteo.kc.aa

import com.intellij.lang.ASTNode
import com.intellij.psi.PsiElement
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotation
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.psi.*
import java.util.IdentityHashMap

/**
 * The symbols of one KSP round: the Go model of the module's sources joined
 * with their PSI. A round is immutable; the next update replaces it and
 * invalidates its [token].
 */
internal class KcRound(files: List<Pair<KcFile, KtFile>>) {
    val token = KcToken()

    /** Every source symbol by its pointer key. */
    val byKey = HashMap<String, KaSymbol>()

    /** Source symbols by their PSI, for `KaSession.symbol`. */
    val byPsi = IdentityHashMap<PsiElement, KaSymbol>()

    val classes = HashMap<ClassId, KaNamedClassSymbol>()
    val typeAliases = HashMap<ClassId, KaTypeAliasSymbol>()
    private val libraryClasses = HashMap<ClassId, KaNamedClassSymbol>()

    /** The class of each annotation entry, for `KtTypeReference.type`. */
    val annotationClasses = IdentityHashMap<KtAnnotationEntry, ClassId>()

    private val annotations = IdentityHashMap<KcAnnotation, KaAnnotation>()

    val fileSymbols: List<KcFileSymbol>

    init {
        fileSymbols = files.map { (f, kt) -> FileBuilder(f, kt).build() }
    }

    fun classSymbol(id: ClassId): KaNamedClassSymbol =
        classes[id] ?: libraryClasses.getOrPut(id) { KcLibraryClassSymbol(this, id) }

    fun annotation(a: KcAnnotation): KaAnnotation = annotations.getValue(a)

    private inner class FileBuilder(val file: KcFile, val kt: KtFile) {
        val nodes = preorder(kt.node)

        fun build(): KcFileSymbol {
            val fs = KcFileSymbol(this@KcRound, file, kt)
            byKey[fs.key] = fs
            byPsi[kt] = fs
            file.annotations.forEach(::annotation)
            fs.declarations = decls(file.decls, fs, fs.key)
            return fs
        }

        fun psi(node: Int): PsiElement? {
            if (node == 0) return null
            check(node < nodes.size) { "kc: node $node outside ${file.path} (${nodes.size - 1} nodes)" }
            return nodes[node].psi
        }

        fun annotation(a: KcAnnotation) {
            val entry = psi(a.node) as? KtAnnotationEntry
                ?: error("kc: annotation node ${a.node} of ${file.path} is ${nodes[a.node].elementType}")
            val id = a.classId.takeIf { it.isNotEmpty() }?.let(ClassId::fromString)
            if (id != null) annotationClasses[entry] = id
            annotations[a] = KcAnnotationApplication(this@KcRound, id, entry, useSiteTarget(a.target))
        }

        fun decls(ds: List<KcDecl>, parent: KaSymbol, parentKey: String): List<KaDeclarationSymbol> =
            ds.mapIndexed { i, d -> decl(d, parent, "$parentKey/${d.kind}:${d.name}:$i") }

        inline fun <reified T : PsiElement> psiOf(d: KcDecl): T? {
            val p = psi(d.node) ?: return null
            return p as? T ?: error("kc: ${d.name} in ${file.path} is ${p.javaClass.simpleName}, want ${T::class.simpleName}")
        }

        fun info(d: KcDecl, psi: PsiElement?, parent: KaSymbol, key: String, annots: List<KcAnnotation> = d.annotations): KcInfo {
            d.annotations.forEach(::annotation)
            return KcInfo(this@KcRound, d, psi, parent, key, annots)
        }

        fun <S : KaSymbol> register(s: S, key: String, psi: PsiElement?): S {
            byKey[key] = s
            if (psi != null) byPsi.putIfAbsent(psi, s)
            return s
        }

        fun decl(d: KcDecl, parent: KaSymbol, key: String): KaDeclarationSymbol = when (d.kind) {
            KcDecl.CLASS, KcDecl.INTERFACE, KcDecl.OBJECT, KcDecl.ENUM_CLASS, KcDecl.ANNOTATION_CLASS -> {
                val psi = psiOf<KtClassOrObject>(d)
                val c = register(KcClassSymbol(info(d, psi, parent, key)), key, psi)
                c.classId?.let { classes[it] = c }
                c.typeParams = typeParams(d, c, key)
                c.members = decls(d.members, c, key)
                c
            }
            KcDecl.ENUM_ENTRY -> {
                val psi = psiOf<KtEnumEntry>(d)
                register(KcEnumEntrySymbol(info(d, psi, parent, key)), key, psi)
            }
            KcDecl.FUNCTION -> {
                val psi = psiOf<KtNamedFunction>(d)
                val f = register(KcFunctionSymbol(info(d, psi, parent, key)), key, psi)
                f.typeParams = typeParams(d, f, key)
                f.params = params(d, f, key)
                f
            }
            KcDecl.CONSTRUCTOR -> {
                val psi = psiOf<KtConstructor<*>>(d)
                val c = register(KcConstructorSymbol(info(d, psi, parent, key)), key, psi)
                c.params = params(d, c, key)
                c
            }
            KcDecl.PROPERTY -> property(d, parent, key)
            KcDecl.TYPE_ALIAS -> {
                val psi = psiOf<KtTypeAlias>(d)
                val t = register(KcTypeAliasSymbol(info(d, psi, parent, key)), key, psi)
                t.classId?.let { typeAliases[it] = t }
                t.typeParams = typeParams(d, t, key)
                t
            }
            else -> error("kc: unexpected declaration kind ${d.kind} for ${d.name} in ${file.path}")
        }

        /**
         * A property, splitting its annotations by use-site target between
         * the property, its accessors and, for a constructor `val`, the
         * parameter (which gets the untargeted ones, as kotlinc does when
         * the annotation may apply to parameters).
         */
        fun property(d: KcDecl, parent: KaSymbol, key: String): KcPropertySymbol {
            val psi = psi(d.node)
            check(psi is KtProperty || psi is KtParameter) { "kc: property ${d.name} in ${file.path} is $psi" }
            val fromCtor = psi is KtParameter
            val own = d.annotations.filter {
                when (it.target) {
                    "get", "set", "setparam", "field", "param" -> false
                    "" -> !fromCtor
                    else -> true
                }
            }
            val p = KcPropertySymbol(info(d, psi, parent, key, own))
            register(p, key, psi)
            p.typeParams = typeParams(d, p, key)
            val accessors = (psi as? KtProperty)?.accessors.orEmpty()
            p.getterSymbol = KcGetterSymbol(
                KcInfo(this@KcRound, d, accessors.firstOrNull { it.isGetter }, p, "$key/get", d.annotations.filter { it.target == "get" }),
                p,
            ).also { register(it, "$key/get", it.info.psi) }
            if (!p.isVal) {
                val setterPsi = accessors.firstOrNull { it.isSetter }
                val s = KcSetterSymbol(
                    KcInfo(this@KcRound, d, setterPsi, p, "$key/set", d.annotations.filter { it.target == "set" }),
                    p,
                )
                register(s, "$key/set", setterPsi)
                val paramDecl = KcDecl(KcDecl.VALUE_PARAMETER, setterPsi?.parameter?.name ?: "value", 0, "",
                    visibility = 0, modality = KcDecl.FINAL, flags = 0, variance = KcDecl.INVARIANT,
                    type = d.type, receiver = null, supertypes = emptyList(), bounds = emptyList(),
                    annotations = emptyList(), typeParams = emptyList(), params = emptyList(), members = emptyList())
                s.parameterSymbol = KcValueParameterSymbol(
                    KcInfo(this@KcRound, paramDecl, setterPsi?.parameter, s, "$key/set/value",
                        d.annotations.filter { it.target == "setparam" }),
                ).also { register(it, "$key/set/value", setterPsi?.parameter) }
                p.setterSymbol = s
            }
            return p
        }

        fun typeParams(d: KcDecl, parent: KaSymbol, key: String): List<KaTypeParameterSymbol> =
            d.typeParams.mapIndexed { i, t ->
                val k = "$key/<$i"
                val psi = psiOf<KtTypeParameter>(t)
                register(KcTypeParameterSymbol(info(t, psi, parent, k)), k, psi)
            }

        fun params(d: KcDecl, parent: KaSymbol, key: String): List<KaValueParameterSymbol> =
            d.params.mapIndexed { i, v ->
                val k = "$key/($i"
                val psi = psiOf<KtParameter>(v)
                val own = v.annotations.filter { it.target == "" || it.target == "param" }
                register(KcValueParameterSymbol(info(v, psi, parent, k, own)), k, psi)
            }
    }

    companion object {
        /**
         * The nodes of a tree in preorder, from 1; index 0 is null. This is
         * the numbering of kc's psi node IDs.
         */
        fun preorder(root: ASTNode): Array<ASTNode> {
            val out = ArrayList<ASTNode?>()
            out.add(null)
            val stack = ArrayDeque<ASTNode>()
            stack.addLast(root)
            while (stack.isNotEmpty()) {
                val n = stack.removeLast()
                out.add(n)
                var c = n.lastChildNode
                while (c != null) {
                    stack.addLast(c)
                    c = c.treePrev
                }
            }
            @Suppress("UNCHECKED_CAST")
            return out.toTypedArray() as Array<ASTNode>
        }
    }
}

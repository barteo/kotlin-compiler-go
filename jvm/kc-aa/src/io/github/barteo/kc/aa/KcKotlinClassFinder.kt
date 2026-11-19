package io.github.barteo.kc.aa

import com.intellij.lang.java.JavaLanguage
import com.intellij.psi.PsiClass
import com.intellij.psi.PsiClassInitializer
import com.intellij.psi.PsiElement
import com.intellij.psi.PsiElementFinder
import com.intellij.psi.PsiField
import com.intellij.psi.PsiJavaFile
import com.intellij.psi.PsiManager
import com.intellij.psi.PsiMethod
import com.intellij.psi.PsiModifier
import com.intellij.psi.PsiModifierList
import com.intellij.psi.PsiPackage
import com.intellij.psi.PsiReferenceList
import com.intellij.psi.PsiTypeParameterList
import com.intellij.psi.impl.PsiModificationTrackerImpl
import com.intellij.psi.impl.file.PsiPackageImpl
import com.intellij.psi.impl.light.LightModifierList
import com.intellij.psi.impl.light.LightPsiClassBase
import com.intellij.psi.search.GlobalSearchScope
import com.intellij.psi.util.PsiModificationTracker
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Finds the module's Kotlin source classes for Java PSI, which resolves the
 * names in Java sources through the project's finders. KSP reads the class
 * of a class literal in a Java annotation's argument (`K.class`) from what
 * the name resolves to, and stock KSP's Java PSI finds Kotlin classes as
 * light classes. The classes found here have names and nested classes
 * only: a member of a Kotlin class stays unresolved in Java PSI.
 *
 * It also finds the classes of the Java sources a processor generated, and
 * the sources' packages: the Java file manager has the Java sources of the
 * source roots only, so it has neither a generated Java source nor a
 * package that only generated sources are in, and a name that an import of
 * such a package brings in would stay unresolved.
 */
internal class KcKotlinClassFinder(private val manager: PsiManager) : PsiElementFinder() {
    private class Classes(val round: KcRound, generatedJava: List<PsiJavaFile>) {
        val psi = HashMap<ClassId, PsiClass>()
        val java = HashMap<ClassId, PsiClass>()
        val byParent: Map<Any, List<ClassId>>

        init {
            fun walk(c: PsiClass, id: ClassId) {
                java[id] = c
                for (n in c.innerClasses) n.name?.let { walk(n, id.createNestedClassId(Name.identifier(it))) }
            }
            for (f in generatedJava) {
                val pkg = FqName(f.packageName)
                for (c in f.classes) c.name?.let { walk(c, ClassId(pkg, Name.identifier(it))) }
            }
            byParent = (round.classes.keys + java.keys).groupBy { it.outerClassId ?: it.packageFqName }
        }
    }

    @Volatile
    private var classes: Classes? = null

    /**
     * Finds the classes of [round], of which [generatedJava] are the Java
     * sources the file manager doesn't index; a null round, between runs,
     * finds none. Java PSI keeps what a name resolved to, so a new round,
     * whose sources may have the class a name didn't find before, drops that.
     */
    fun find(round: KcRound?, generatedJava: List<PsiJavaFile> = emptyList()) {
        if (round === classes?.round) return
        classes = round?.let { Classes(it, generatedJava) }
        if (round != null) {
            manager.dropResolveCaches()
            (PsiModificationTracker.getInstance(manager.project) as PsiModificationTrackerImpl).incCounter()
        }
    }

    fun psiClass(id: ClassId): PsiClass? {
        val c = classes ?: return null
        c.java[id]?.let { return it }
        if (!c.round.classes.containsKey(id)) return null
        return synchronized(c) { c.psi.getOrPut(id) { KcKotlinPsiClass(this, id, manager) } }
    }

    /** The classes nested in class [parent] (a [ClassId]) or at the top of package [parent] (a [FqName]). */
    fun classesIn(parent: Any): Array<PsiClass> =
        classes?.byParent?.get(parent)?.mapNotNull(::psiClass)?.toTypedArray() ?: PsiClass.EMPTY_ARRAY

    override fun findClass(qualifiedName: String, scope: GlobalSearchScope): PsiClass? {
        if (classes == null) return null
        val segs = qualifiedName.split('.')
        for (i in segs.size - 1 downTo 0) {
            psiClass(ClassId(FqName.fromSegments(segs.subList(0, i)), FqName.fromSegments(segs.subList(i, segs.size)), false))
                ?.let { return it }
        }
        return null
    }

    override fun findClasses(qualifiedName: String, scope: GlobalSearchScope): Array<PsiClass> =
        findClass(qualifiedName, scope)?.let { arrayOf(it) } ?: PsiClass.EMPTY_ARRAY

    override fun getClasses(psiPackage: PsiPackage, scope: GlobalSearchScope): Array<PsiClass> =
        classesIn(FqName(psiPackage.qualifiedName))

    override fun findPackage(qualifiedName: String): PsiPackage? =
        if (classes?.round?.hasSources(FqName(qualifiedName)) == true) PsiPackageImpl(manager, qualifiedName) else null
}

private class KcKotlinPsiClass(private val finder: KcKotlinClassFinder, private val id: ClassId, manager: PsiManager) :
    LightPsiClassBase(manager, JavaLanguage.INSTANCE, id.shortClassName.asString()) {
    private val modifiers = LightModifierList(manager, JavaLanguage.INSTANCE, PsiModifier.PUBLIC)

    override fun getQualifiedName(): String = id.asSingleFqName().asString()
    override fun getModifierList(): PsiModifierList = modifiers
    override fun getContainingClass(): PsiClass? = id.outerClassId?.let(finder::psiClass)
    override fun getScope(): PsiElement? = containingClass
    override fun getInnerClasses(): Array<PsiClass> = finder.classesIn(id)
    override fun findInnerClassByName(name: String, checkBases: Boolean): PsiClass? =
        finder.psiClass(id.createNestedClassId(Name.identifier(name)))
    override fun getExtendsList(): PsiReferenceList? = null
    override fun getImplementsList(): PsiReferenceList? = null
    override fun getTypeParameterList(): PsiTypeParameterList? = null
    override fun getFields(): Array<PsiField> = PsiField.EMPTY_ARRAY
    override fun getMethods(): Array<PsiMethod> = PsiMethod.EMPTY_ARRAY
    override fun getInitializers(): Array<PsiClassInitializer> = PsiClassInitializer.EMPTY_ARRAY
    override fun toString() = "KcKotlinPsiClass($id)"
}

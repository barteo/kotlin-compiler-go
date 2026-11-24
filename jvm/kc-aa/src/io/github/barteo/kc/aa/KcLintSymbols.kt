@file:Suppress("DEPRECATION", "DEPRECATION_ERROR", "OVERRIDE_DEPRECATION")

package io.github.barteo.kc.aa

import com.intellij.psi.PsiElement
import org.jetbrains.kotlin.analysis.api.KaSession
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotation
import org.jetbrains.kotlin.analysis.api.annotations.KaAnnotationList
import org.jetbrains.kotlin.analysis.api.annotations.KaNamedAnnotationValue
import org.jetbrains.kotlin.analysis.api.base.KaContextReceiver
import org.jetbrains.kotlin.analysis.api.symbols.*
import org.jetbrains.kotlin.analysis.api.symbols.pointers.KaSymbolPointer
import org.jetbrains.kotlin.analysis.api.types.KaType
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.descriptors.Visibility
import org.jetbrains.kotlin.descriptors.annotations.AnnotationUseSiteTarget
import org.jetbrains.kotlin.lexer.KtModifierKeywordToken
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.SpecialNames
import org.jetbrains.kotlin.psi.*
import org.jetbrains.kotlin.psi.psiUtil.containingClassOrObject
import org.jetbrains.kotlin.psi.psiUtil.getParentOfType
import org.jetbrains.kotlin.psi.psiUtil.isTopLevelKtOrJavaMember
import org.jetbrains.kotlin.types.Variance

/** A pointer that restores the symbol it was made from: a linter's symbols live as long as its session. */
internal class KcLintPointer<S : KaSymbol>(private val symbol: S) : StubKaSymbolPointer<S>() {
    override fun restoreSymbol(analysisSession: KaSession): S = symbol
}

// What the PSI of a source declaration tells.

private fun KtModifierListOwner.has(t: KtModifierKeywordToken) = hasModifier(t)

internal fun lintVisibility(d: KtDeclaration): KaSymbolVisibility = when {
    d.has(KtTokens.PRIVATE_KEYWORD) -> KaSymbolVisibility.PRIVATE
    d.has(KtTokens.PROTECTED_KEYWORD) -> KaSymbolVisibility.PROTECTED
    d.has(KtTokens.INTERNAL_KEYWORD) -> KaSymbolVisibility.INTERNAL
    d is KtProperty && d.isLocal || d is KtFunction && d.isLocal -> KaSymbolVisibility.LOCAL
    d.parent is KtClassBody && (d.parent.parent as? KtClassOrObject)?.let { it is KtObjectDeclaration && it.isObjectLiteral() } == true -> KaSymbolVisibility.LOCAL
    else -> KaSymbolVisibility.PUBLIC
}

internal fun lintCompilerVisibility(v: KaSymbolVisibility): Visibility = when (v) {
    KaSymbolVisibility.PRIVATE -> Visibilities.Private
    KaSymbolVisibility.PROTECTED -> Visibilities.Protected
    KaSymbolVisibility.INTERNAL -> Visibilities.Internal
    KaSymbolVisibility.LOCAL -> Visibilities.Local
    else -> Visibilities.Public
}

/** A member's modality: abstract, open (open or an override that isn't final), or final. */
internal fun lintMemberModality(d: KtCallableDeclaration): KaSymbolModality {
    val inInterface = (d.containingClassOrObject as? KtClass)?.isInterface() == true
    val hasBody = when (d) {
        is KtNamedFunction -> d.hasBody()
        is KtProperty -> d.initializer != null || d.delegate != null || d.accessors.any { it.hasBody() }
        else -> true
    }
    return when {
        d.has(KtTokens.ABSTRACT_KEYWORD) || inInterface && !hasBody -> KaSymbolModality.ABSTRACT
        d.has(KtTokens.FINAL_KEYWORD) -> KaSymbolModality.FINAL
        d.has(KtTokens.OPEN_KEYWORD) || inInterface || d.has(KtTokens.OVERRIDE_KEYWORD) && d.containingClassOrObject?.let(::lintClassModality) != KaSymbolModality.FINAL ->
            KaSymbolModality.OPEN
        else -> KaSymbolModality.FINAL
    }
}

internal fun lintClassModality(c: KtClassOrObject): KaSymbolModality = when {
    c is KtClass && c.isInterface() -> KaSymbolModality.ABSTRACT
    c.has(KtTokens.SEALED_KEYWORD) -> KaSymbolModality.SEALED
    c.has(KtTokens.ABSTRACT_KEYWORD) -> KaSymbolModality.ABSTRACT
    c.has(KtTokens.OPEN_KEYWORD) -> KaSymbolModality.OPEN
    else -> KaSymbolModality.FINAL
}

internal fun lintLocation(d: KtDeclaration): KaSymbolLocation = when {
    d.isTopLevelKtOrJavaMember() -> KaSymbolLocation.TOP_LEVEL
    d.parent is KtClassBody || d is KtParameter && d.hasValOrVar() -> KaSymbolLocation.CLASS
    else -> KaSymbolLocation.LOCAL
}

/** The callable id of a source callable [d] named [name]. */
internal fun lintCallableId(d: KtDeclaration, name: Name): CallableId? {
    if (lintLocation(d) == KaSymbolLocation.LOCAL) return null
    val cls = (if (d is KtParameter) d.getParentOfType<KtClassOrObject>(true) else d.containingClassOrObject)?.getClassId()
    return if (cls != null) CallableId(cls, name) else CallableId(d.containingKtFile.packageFqName, name)
}

/** The annotations written on a declaration, with their classes as kc resolved them. */
internal object KcLintAnnotations {
    fun of(session: KcLint, d: KtAnnotated?): KaAnnotationList {
        val entries = d?.annotationEntries.orEmpty()
        if (entries.isEmpty()) return session.noAnnotations
        return KcAnnotationList(entries.map { KcLintAnnotation(session, annotationClassId(session, it), it) }, session.token)
    }

    fun ofNames(session: KcLint, names: List<String>): KaAnnotationList =
        if (names.isEmpty()) session.noAnnotations else KcAnnotationList(names.map { KcLintAnnotation(session, KcLint.fqClassId(it), null) }, session.token)

    fun annotationClassId(session: KcLint, e: KtAnnotationEntry): ClassId? {
        val ref = e.typeReference ?: return null
        val t = session.fact(ref)?.type ?: session.fact(ref.typeElement)?.type
        if (t != null && t.classId.isNotEmpty()) return runCatching { ClassId.fromString(t.classId) }.getOrNull()
        return null
    }
}

internal class KcLintAnnotation(private val session: KcLint, override val classId: ClassId?, override val psi: KtAnnotationEntry?) : StubKaAnnotation() {
    override val token get() = session.token
    override val useSiteTarget: AnnotationUseSiteTarget? get() = psi?.useSiteTarget?.getAnnotationUseSiteTarget()
    override val arguments: List<KaNamedAnnotationValue> get() = KcMissing.miss("KaAnnotation.arguments", emptyList())
    override val constructorSymbol: KaConstructorSymbol? get() = null
    override fun equals(other: Any?) = other is KcLintAnnotation && other.classId == classId && other.psi == psi
    override fun hashCode() = classId.hashCode()
}

/** The type kc resolved for type reference [ref]. */
internal fun lintTypeOfRef(session: KcLint, ref: KtTypeReference?): KaType? {
    if (ref == null) return null
    val t = session.fact(ref)?.type ?: session.fact(ref.typeElement)?.type ?: return null
    return session.type(t)
}

/** The type of the symbol kc declares at [d], or its written type, or [fallback]. */
internal fun lintDeclaredType(session: KcLint, d: KtCallableDeclaration, fallback: () -> KaType): KaType {
    session.sym(session.fact(d)?.symbol ?: 0)?.type?.let { return session.type(it) }
    lintTypeOfRef(session, d.typeReference)?.let { return it }
    return fallback()
}

// Source declarations.

internal class KcLintClassSymbol(private val session: KcLint, private val c: KtClassOrObject) : StubKaNamedClassSymbol() {
    override val token get() = session.token
    override val psi: PsiElement get() = c
    override val origin get() = KaSymbolOrigin.SOURCE
    override val classId: ClassId? get() = c.getClassId()
    override val name: Name get() = c.nameAsName ?: SpecialNames.NO_NAME_PROVIDED
    override val classKind: KaClassKind
        get() = when {
            c is KtObjectDeclaration -> if (c.isCompanion()) KaClassKind.COMPANION_OBJECT else KaClassKind.OBJECT
            c is KtClass && c.isAnnotation() -> KaClassKind.ANNOTATION_CLASS
            c is KtClass && c.isEnum() -> KaClassKind.ENUM_CLASS
            c is KtClass && c.isInterface() -> KaClassKind.INTERFACE
            else -> KaClassKind.CLASS
        }
    override val modality: KaSymbolModality get() = lintClassModality(c)
    override val visibility: KaSymbolVisibility get() = lintVisibility(c)
    override val compilerVisibility: Visibility get() = lintCompilerVisibility(visibility)
    override val location: KaSymbolLocation get() = lintLocation(c)
    override val superTypes: List<KaType>
        get() = c.superTypeListEntries.mapNotNull { lintTypeOfRef(session, it.typeReference) }.ifEmpty {
            listOf(if (c is KtClass && c.isEnum()) session.classType("kotlin/Enum", false) else session.any)
        }
    override val typeParameters: List<KaTypeParameterSymbol>
        get() = c.typeParameters.mapNotNull { session.declarationSymbol(it) as? KaTypeParameterSymbol }
    override val companionObject: KaNamedClassSymbol?
        get() = c.companionObjects.firstOrNull()?.let { session.declarationSymbol(it) as? KaNamedClassSymbol }
    override val isData: Boolean get() = c.has(KtTokens.DATA_KEYWORD)
    override val isInline: Boolean get() = c.has(KtTokens.INLINE_KEYWORD) || c.has(KtTokens.VALUE_KEYWORD)
    override val isFun: Boolean get() = c.has(KtTokens.FUN_KEYWORD)
    override val isInner: Boolean get() = c.has(KtTokens.INNER_KEYWORD)
    override val isExternal: Boolean get() = c.has(KtTokens.EXTERNAL_KEYWORD)
    override val isExpect: Boolean get() = c.has(KtTokens.EXPECT_KEYWORD)
    override val isActual: Boolean get() = c.has(KtTokens.ACTUAL_KEYWORD)
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override val annotations: KaAnnotationList get() = KcLintAnnotations.of(session, c)
    override fun createPointer(): KaSymbolPointer<KaNamedClassSymbol> = KcLintPointer(this)
    override fun toString() = "class ${classId ?: name}"
}

internal class KcLintAnonymousObjectSymbol(private val session: KcLint, private val o: KtObjectDeclaration) : StubKaAnonymousObjectSymbol() {
    override val token get() = session.token
    override val psi: PsiElement get() = o
    override val origin get() = KaSymbolOrigin.SOURCE
    override val superTypes: List<KaType> get() = o.superTypeListEntries.mapNotNull { lintTypeOfRef(session, it.typeReference) }.ifEmpty { listOf(session.any) }
    override val annotations: KaAnnotationList get() = session.noAnnotations
    override fun createPointer(): KaSymbolPointer<KaAnonymousObjectSymbol> = KcLintPointer(this)
}

internal class KcLintFunctionSymbol(private val session: KcLint, private val f: KtNamedFunction) : StubKaNamedFunctionSymbol() {
    override val token get() = session.token
    override val psi: PsiElement get() = f
    override val origin get() = KaSymbolOrigin.SOURCE
    override val name: Name get() = f.nameAsSafeName
    override val callableId: CallableId? get() = lintCallableId(f, name)
    override val returnType: KaType
        get() = lintDeclaredType(session, f) { if (f.hasBlockBody() || !f.hasBody()) session.unit else KcErrorType(session.round, "return type of ${f.name}") }
    override val valueParameters: List<KaValueParameterSymbol>
        get() = f.valueParameters.mapNotNull { session.declarationSymbol(it) as? KaValueParameterSymbol }
    override val typeParameters: List<KaTypeParameterSymbol>
        get() = f.typeParameters.mapNotNull { session.declarationSymbol(it) as? KaTypeParameterSymbol }
    override val receiverParameter: KaReceiverParameterSymbol?
        get() = f.receiverTypeReference?.let { r -> KcLintReceiverParameterSymbol(this, lintTypeOfRef(session, r) ?: KcErrorType(session.round, r.text), r) }
    override val isExtension: Boolean get() = f.receiverTypeReference != null
    override val isSuspend: Boolean get() = f.has(KtTokens.SUSPEND_KEYWORD)
    override val isInline: Boolean get() = f.has(KtTokens.INLINE_KEYWORD)
    override val isOperator: Boolean get() = f.has(KtTokens.OPERATOR_KEYWORD)
    override val isInfix: Boolean get() = f.has(KtTokens.INFIX_KEYWORD)
    override val isOverride: Boolean get() = f.has(KtTokens.OVERRIDE_KEYWORD)
    override val isTailRec: Boolean get() = f.has(KtTokens.TAILREC_KEYWORD)
    override val isExternal: Boolean get() = f.has(KtTokens.EXTERNAL_KEYWORD)
    override val isExpect: Boolean get() = f.has(KtTokens.EXPECT_KEYWORD)
    override val isActual: Boolean get() = f.has(KtTokens.ACTUAL_KEYWORD)
    override val isStatic: Boolean get() = false
    override val isBuiltinFunctionInvoke: Boolean get() = false
    override val hasStableParameterNames: Boolean get() = true
    override val modality: KaSymbolModality get() = lintMemberModality(f)
    override val visibility: KaSymbolVisibility get() = lintVisibility(f)
    override val compilerVisibility: Visibility get() = lintCompilerVisibility(visibility)
    override val location: KaSymbolLocation get() = lintLocation(f)
    override val contextParameters: List<KaContextParameterSymbol> get() = emptyList()
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override val annotations: KaAnnotationList get() = KcLintAnnotations.of(session, f)
    override fun createPointer(): KaSymbolPointer<KaNamedFunctionSymbol> = KcLintPointer(this)
    override fun toString() = "fun ${callableId ?: name}"
}

internal class KcLintAnonymousFunctionSymbol(private val session: KcLint, private val f: KtFunction) : StubKaAnonymousFunctionSymbol() {
    override val token get() = session.token
    override val psi: PsiElement get() = f
    override val origin get() = KaSymbolOrigin.SOURCE
    private val lambdaType: KaType?
        get() = (f.parent as? KtLambdaExpression ?: f).let { e -> session.fact(e)?.type }?.let { session.type(it) }
    override val returnType: KaType
        get() = (lambdaType as? KcFunctionType)?.returnType ?: lintTypeOfRef(session, f.typeReference) ?: session.unit
    override val valueParameters: List<KaValueParameterSymbol>
        get() {
            val written = f.valueParameters.mapNotNull { session.declarationSymbol(it) as? KaValueParameterSymbol }
            if (written.isNotEmpty() || f !is KtFunctionLiteral) return written
            // The implicit it.
            val fact = session.fact(f.parent) ?: return emptyList()
            if (!fact.implicitParameter) return emptyList()
            val it = session.sym(fact.lambdaParameters?.firstOrNull() ?: 0) ?: return emptyList()
            return listOf(KcLintImplicitParameterSymbol(session, this, it))
        }
    // A lambda's receiver, at the lambda, as FIR's is.
    override val receiverParameter: KaReceiverParameterSymbol?
        get() = f.receiverTypeReference?.let { r -> KcLintReceiverParameterSymbol(this, lintTypeOfRef(session, r) ?: KcErrorType(session.round, r.text), r) }
            ?: (lambdaType as? KcFunctionType)?.receiverType?.takeIf { f is KtFunctionLiteral }?.let { KcLintReceiverParameterSymbol(this, it, f) }
    override val isExtension: Boolean get() = receiverParameter != null
    override val visibility: KaSymbolVisibility get() = KaSymbolVisibility.LOCAL
    override val compilerVisibility: Visibility get() = Visibilities.Local
    override val contextParameters: List<KaContextParameterSymbol> get() = emptyList()
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override val annotations: KaAnnotationList get() = session.noAnnotations
    override fun createPointer(): KaSymbolPointer<KaAnonymousFunctionSymbol> = KcLintPointer(this)
}

internal class KcLintLocalVariableSymbol(private val session: KcLint, private val v: KtCallableDeclaration) : StubKaLocalVariableSymbol() {
    override val token get() = session.token
    override val psi: PsiElement get() = v
    override val origin get() = KaSymbolOrigin.SOURCE
    override val name: Name get() = v.nameAsSafeName
    override val returnType: KaType get() = lintDeclaredType(session, v) { KcErrorType(session.round, "type of ${v.name}") }
    override val isVal: Boolean get() = (v as? KtValVarKeywordOwner)?.valOrVarKeyword?.node?.elementType != KtTokens.VAR_KEYWORD &&
        (v as? KtDestructuringDeclarationEntry)?.let { (it.parent as? KtDestructuringDeclaration)?.isVar } != true
    override val isLateInit: Boolean get() = v.has(KtTokens.LATEINIT_KEYWORD)
    override val annotations: KaAnnotationList get() = KcLintAnnotations.of(session, v)
    override fun createPointer(): KaSymbolPointer<KaLocalVariableSymbol> = KcLintPointer(this)
    override fun toString() = "local ${v.name}"
}

internal class KcLintPropertySymbol(private val session: KcLint, private val p: KtProperty) : StubKaKotlinPropertySymbol() {
    override val token get() = session.token
    override val psi: PsiElement get() = p
    override val origin get() = KaSymbolOrigin.SOURCE
    override val name: Name get() = p.nameAsSafeName
    override val callableId: CallableId? get() = lintCallableId(p, name)
    override val returnType: KaType get() = lintDeclaredType(session, p) { KcErrorType(session.round, "type of ${p.name}") }
    override val isVal: Boolean get() = !p.isVar
    override val hasGetter: Boolean get() = true
    override val hasSetter: Boolean get() = p.isVar
    override val getter: KaPropertyGetterSymbol by lazy(LazyThreadSafetyMode.PUBLICATION) { KcLintGetterSymbol(session, this, p.getter) }
    override val setter: KaPropertySetterSymbol? by lazy(LazyThreadSafetyMode.PUBLICATION) { if (p.isVar) KcLintSetterSymbol(session, this, p.setter) else null }
    override val hasBackingField: Boolean
        get() = p.initializer != null || p.has(KtTokens.LATEINIT_KEYWORD) ||
            !p.hasDelegate() && (p.getter?.hasBody() != true || p.setter?.hasBody() == false) &&
            (p.containingClassOrObject as? KtClass)?.isInterface() != true && !p.has(KtTokens.ABSTRACT_KEYWORD)
    override val backingFieldSymbol: KaBackingFieldSymbol? get() = null
    override val receiverParameter: KaReceiverParameterSymbol?
        get() = p.receiverTypeReference?.let { r -> KcLintReceiverParameterSymbol(this, lintTypeOfRef(session, r) ?: KcErrorType(session.round, r.text), r) }
    override val isExtension: Boolean get() = p.receiverTypeReference != null
    override val typeParameters: List<KaTypeParameterSymbol>
        get() = p.typeParameters.mapNotNull { session.declarationSymbol(it) as? KaTypeParameterSymbol }
    override val isConst: Boolean get() = p.has(KtTokens.CONST_KEYWORD)
    override val isLateInit: Boolean get() = p.has(KtTokens.LATEINIT_KEYWORD)
    override val isOverride: Boolean get() = p.has(KtTokens.OVERRIDE_KEYWORD)
    override val isExternal: Boolean get() = p.has(KtTokens.EXTERNAL_KEYWORD)
    override val isExpect: Boolean get() = p.has(KtTokens.EXPECT_KEYWORD)
    override val isActual: Boolean get() = p.has(KtTokens.ACTUAL_KEYWORD)
    override val isStatic: Boolean get() = false
    override val isFromPrimaryConstructor: Boolean get() = false
    override val initializer: org.jetbrains.kotlin.analysis.api.KaInitializerValue? get() = null
    override val modality: KaSymbolModality get() = lintMemberModality(p)
    override val visibility: KaSymbolVisibility get() = lintVisibility(p)
    override val compilerVisibility: Visibility get() = lintCompilerVisibility(visibility)
    override val location: KaSymbolLocation get() = lintLocation(p)
    override val contextParameters: List<KaContextParameterSymbol> get() = emptyList()
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override val annotations: KaAnnotationList get() = KcLintAnnotations.of(session, p)
    override fun createPointer(): KaSymbolPointer<KaKotlinPropertySymbol> = KcLintPointer(this)
    override fun toString() = "val ${callableId ?: name}"
}

internal class KcLintGetterSymbol(private val session: KcLint, private val property: KcLintPropertySymbol, private val a: KtPropertyAccessor?) :
    StubKaPropertyGetterSymbol() {
    override val token get() = session.token
    override val psi: PsiElement? get() = a
    override val origin get() = KaSymbolOrigin.SOURCE
    override val callableId: CallableId? get() = null
    override val returnType: KaType get() = property.returnType
    override val hasBody: Boolean get() = a?.hasBody() == true
    override val isDefault: Boolean get() = a == null
    override val isNotDefault: Boolean get() = a != null
    override val isInline: Boolean get() = a?.hasModifier(KtTokens.INLINE_KEYWORD) == true
    override val isExternal: Boolean get() = false
    override val isOverride: Boolean get() = property.isOverride
    override val modality: KaSymbolModality get() = property.modality
    override val visibility: KaSymbolVisibility get() = property.visibility
    override val compilerVisibility: Visibility get() = property.compilerVisibility
    override val receiverParameter: KaReceiverParameterSymbol? get() = property.receiverParameter
    override val annotations: KaAnnotationList get() = KcLintAnnotations.of(session, a)
    override fun createPointer(): KaSymbolPointer<KaPropertyGetterSymbol> = KcLintPointer(this)
}

internal class KcLintSetterSymbol(private val session: KcLint, private val property: KcLintPropertySymbol, private val a: KtPropertyAccessor?) :
    StubKaPropertySetterSymbol() {
    override val token get() = session.token
    override val psi: PsiElement? get() = a
    override val origin get() = KaSymbolOrigin.SOURCE
    override val callableId: CallableId? get() = null
    override val returnType: KaType get() = session.unit
    override val hasBody: Boolean get() = a?.hasBody() == true
    override val hasStableParameterNames: Boolean get() = true
    override val isDefault: Boolean get() = a == null
    override val isNotDefault: Boolean get() = a != null
    override val isInline: Boolean get() = false
    override val isExternal: Boolean get() = false
    override val isOverride: Boolean get() = property.isOverride
    override val modality: KaSymbolModality get() = property.modality
    override val visibility: KaSymbolVisibility
        get() = a?.let { if (it.hasModifier(KtTokens.PRIVATE_KEYWORD)) KaSymbolVisibility.PRIVATE else null } ?: property.visibility
    override val compilerVisibility: Visibility get() = lintCompilerVisibility(visibility)
    override val parameter: KaValueParameterSymbol
        get() = a?.parameter?.let { session.declarationSymbol(it) as? KaValueParameterSymbol }
            ?: KcLintLibraryValueParameterSymbol(session, Name.identifier("value"), property.returnType, false, false, false)
    override val receiverParameter: KaReceiverParameterSymbol? get() = property.receiverParameter
    override val annotations: KaAnnotationList get() = KcLintAnnotations.of(session, a)
    override fun createPointer(): KaSymbolPointer<KaPropertySetterSymbol> = KcLintPointer(this)
}

internal class KcLintValueParameterSymbol(private val session: KcLint, private val p: KtParameter) : StubKaValueParameterSymbol() {
    override val token get() = session.token
    override val psi: PsiElement get() = p
    override val origin get() = KaSymbolOrigin.SOURCE
    override val name: Name get() = p.nameAsSafeName
    override val returnType: KaType
        get() {
            if (p.isVarArg) lintTypeOfRef(session, p.typeReference)?.let { return it }
            val t = lintDeclaredType(session, p) { KcErrorType(session.round, "type of ${p.name}") }
            // kc types a vararg parameter as its array.
            if (p.isVarArg && t is KcClassType && t.classId.asFqNameString() == "kotlin.Array") {
                t.typeArguments.firstOrNull()?.type?.let { return it }
            }
            return t
        }
    override val isVararg: Boolean get() = p.isVarArg
    override val hasDefaultValue: Boolean get() = p.hasDefaultValue()
    override val hasDeclaredDefaultValue: Boolean get() = p.hasDefaultValue()
    override val isCrossinline: Boolean get() = p.has(KtTokens.CROSSINLINE_KEYWORD)
    override val isNoinline: Boolean get() = p.has(KtTokens.NOINLINE_KEYWORD)
    override val isImplicitLambdaParameter: Boolean get() = false
    override val visibility: KaSymbolVisibility get() = KaSymbolVisibility.LOCAL
    override val compilerVisibility: Visibility get() = Visibilities.Local
    override val generatedPrimaryConstructorProperty: KaKotlinPropertySymbol?
        get() = if (p.hasValOrVar()) KcMissing.miss("KaValueParameterSymbol.generatedPrimaryConstructorProperty", null) else null
    override val annotations: KaAnnotationList get() = KcLintAnnotations.of(session, p)
    override fun createPointer(): KaSymbolPointer<KaValueParameterSymbol> = KcLintPointer(this)
    override fun toString() = "param ${p.name}"
}

/** A lambda's implicit it. */
internal class KcLintImplicitParameterSymbol(private val session: KcLint, @Suppress("unused") private val owner: KaSymbol?, private val sym: KcFSym) : StubKaValueParameterSymbol() {
    override val token get() = session.token
    override val psi: PsiElement? get() = null
    override val origin get() = KaSymbolOrigin.SOURCE
    override val name: Name get() = Name.identifier("it")
    override val returnType: KaType get() = session.type(sym.type)
    override val isVararg get() = false
    override val hasDefaultValue get() = false
    override val hasDeclaredDefaultValue get() = false
    override val isCrossinline get() = false
    override val isNoinline get() = false
    override val isImplicitLambdaParameter get() = true
    override val visibility get() = KaSymbolVisibility.LOCAL
    override val compilerVisibility: Visibility get() = Visibilities.Local
    override val annotations: KaAnnotationList get() = session.noAnnotations
    override fun createPointer(): KaSymbolPointer<KaValueParameterSymbol> = KcLintPointer(this)
    override fun equals(other: Any?) = other is KcLintImplicitParameterSymbol && other.sym.id == sym.id
    override fun hashCode() = sym.id
}

internal class KcLintDestructuringSymbol(private val session: KcLint, private val d: KtDestructuringDeclaration) : StubKaDestructuringDeclarationSymbol() {
    override val token get() = session.token
    override val psi: PsiElement get() = d
    override val origin get() = KaSymbolOrigin.SOURCE
    override val entries: List<KaVariableSymbol> get() = d.entries.mapNotNull { session.declarationSymbol(it) as? KaVariableSymbol }
    override val annotations: KaAnnotationList get() = session.noAnnotations
    override fun createPointer(): KaSymbolPointer<KaDestructuringDeclarationSymbol> = KcLintPointer(this)
}

internal class KcLintConstructorSymbol(private val session: KcLint, private val c: KtConstructor<*>) : StubKaConstructorSymbol() {
    override val token get() = session.token
    override val psi: PsiElement get() = c
    override val origin get() = KaSymbolOrigin.SOURCE
    override val containingClassId: ClassId? get() = c.getContainingClassOrObject().getClassId()
    override val returnType: KaType
        get() = containingClassId?.let { id -> session.classType(id.asString(), false) } ?: KcErrorType(session.round, "constructor")
    override val valueParameters: List<KaValueParameterSymbol>
        get() = c.valueParameters.mapNotNull { session.declarationSymbol(it) as? KaValueParameterSymbol }
    override val typeParameters: List<KaTypeParameterSymbol> get() = emptyList()
    override val isPrimary: Boolean get() = c is KtPrimaryConstructor
    override val hasStableParameterNames: Boolean get() = true
    override val isExpect get() = false
    override val isActual get() = c.hasModifier(KtTokens.ACTUAL_KEYWORD)
    override val isExternal get() = false
    override val visibility: KaSymbolVisibility get() = lintVisibility(c)
    override val compilerVisibility: Visibility get() = lintCompilerVisibility(visibility)
    override val annotations: KaAnnotationList get() = KcLintAnnotations.of(session, c)
    override fun createPointer(): KaSymbolPointer<KaConstructorSymbol> = KcLintPointer(this)
}

internal class KcLintTypeParameterSymbol(private val session: KcLint, private val p: KtTypeParameter) : StubKaTypeParameterSymbol() {
    override val token get() = session.token
    override val psi: PsiElement get() = p
    override val origin get() = KaSymbolOrigin.SOURCE
    override val name: Name get() = p.nameAsSafeName
    override val isReified: Boolean get() = p.hasModifier(KtTokens.REIFIED_KEYWORD)
    override val variance: Variance get() = p.variance
    override val upperBounds: List<KaType> get() = listOfNotNull(lintTypeOfRef(session, p.extendsBound))
    override val location: KaSymbolLocation get() = KaSymbolLocation.LOCAL
    override val annotations: KaAnnotationList get() = session.noAnnotations
    override fun createPointer(): KaSymbolPointer<KaTypeParameterSymbol> = KcLintPointer(this)
}

internal class KcLintTypeAliasSymbol(private val session: KcLint, private val a: KtTypeAlias) : StubKaTypeAliasSymbol() {
    override val token get() = session.token
    override val psi: PsiElement get() = a
    override val origin get() = KaSymbolOrigin.SOURCE
    override val classId: ClassId? get() = a.getClassId()
    override val name: Name get() = a.nameAsSafeName
    override val expandedType: KaType get() = lintTypeOfRef(session, a.getTypeReference()) ?: KcErrorType(session.round, "expansion of ${a.name}")
    override val typeParameters: List<KaTypeParameterSymbol> get() = a.typeParameters.mapNotNull { session.declarationSymbol(it) as? KaTypeParameterSymbol }
    override val isExpect get() = false
    override val isActual get() = a.hasModifier(KtTokens.ACTUAL_KEYWORD)
    override val visibility: KaSymbolVisibility get() = lintVisibility(a)
    override val compilerVisibility: Visibility get() = lintCompilerVisibility(visibility)
    override val location: KaSymbolLocation get() = lintLocation(a)
    override val annotations: KaAnnotationList get() = KcLintAnnotations.of(session, a)
    override fun createPointer(): KaSymbolPointer<KaTypeAliasSymbol> = KcLintPointer(this)
}

internal class KcLintReceiverParameterSymbol(override val owningCallableSymbol: KaCallableSymbol, override val returnType: KaType, override val psi: PsiElement?) :
    StubKaReceiverParameterSymbol() {
    override val token get() = owningCallableSymbol.token
    override val origin get() = owningCallableSymbol.origin
    override val visibility get() = KaSymbolVisibility.PUBLIC
    override val compilerVisibility: Visibility get() = Visibilities.Public
    override val annotations: KaAnnotationList get() = KcAnnotationList(emptyList(), token)
    override fun createPointer(): KaSymbolPointer<KaReceiverParameterSymbol> = KcLintPointer(this)
}

// Library declarations: what kc's facts tell of the callees the sources use.
// Their classes are the round's (KcRound.classSymbol).

private fun lintLibraryOrigin(fromJava: Boolean) = if (fromJava) KaSymbolOrigin.JAVA_LIBRARY else KaSymbolOrigin.LIBRARY

private fun javaClass(id: ClassId) = id.packageFqName.asString().let { it == "java" || it.startsWith("java.") || it.startsWith("javax.") }



internal class KcLintLibraryValueParameterSymbol(
    private val session: KcLint,
    override val name: Name,
    override val returnType: KaType,
    override val isVararg: Boolean,
    override val isCrossinline: Boolean,
    override val isNoinline: Boolean,
    private val fromJava: Boolean = false,
) : StubKaValueParameterSymbol() {
    override val token get() = session.token
    override val psi: PsiElement? get() = null
    override val origin get() = lintLibraryOrigin(fromJava)
    override val hasDefaultValue get() = KcMissing.miss("library KaValueParameterSymbol.hasDefaultValue", false)
    override val hasDeclaredDefaultValue get() = hasDefaultValue
    override val isImplicitLambdaParameter get() = false
    override val visibility get() = KaSymbolVisibility.LOCAL
    override val compilerVisibility: Visibility get() = Visibilities.Local
    override val annotations: KaAnnotationList get() = session.noAnnotations
    override fun createPointer(): KaSymbolPointer<KaValueParameterSymbol> = KcLintPointer(this)
    override fun toString() = "param $name"
}

/** The element type of a vararg parameter kc types as its array. */
internal fun lintVarargElement(session: KcLint, t: KcFType): KaType {
    if (t.classId == "kotlin/Array") t.args.firstOrNull()?.let { return session.type(it) }
    primitiveArrays[t.classId]?.let { return session.classType(it, false) }
    return session.type(t)
}

private val primitiveArrays = listOf("Int", "Long", "Short", "Byte", "Char", "Boolean", "Float", "Double")
    .associate { "kotlin/${it}Array" to "kotlin/$it" }

/** The callable id of a library callee named [name], a.b.C.f or a.b.f, a member of class [owner] if it isn't "". */
internal fun lintLibraryCallableId(name: String, owner: String): CallableId {
    val simple = name.substringAfterLast('.')
    return if (owner.isNotEmpty()) CallableId(runCatching { ClassId.fromString(owner) }.getOrElse { KcLint.fqClassId(name.substringBeforeLast('.')) }, Name.guessByFirstCharacter(simple))
    else CallableId(FqName(name.substringBeforeLast('.', "")), Name.guessByFirstCharacter(simple))
}

internal class KcLintLibraryFunctionSymbol(private val session: KcLint, private val call: KcFCall?, private val sym: KcFSym?) : StubKaNamedFunctionSymbol() {
    override val token get() = session.token
    override val psi: PsiElement? get() = null
    // FIR's call of an inherited member under a substituted signature is of
    // the substitution override, not the declaration.
    override val origin get() = if (call?.substituted == true) KaSymbolOrigin.SUBSTITUTION_OVERRIDE else lintLibraryOrigin(call?.fromJava == true)
    private val fqName: String get() = call?.name ?: sym?.qualifiedName.orEmpty().ifEmpty { sym?.name.orEmpty() }
    override val name: Name get() = Name.guessByFirstCharacter(fqName.substringAfterLast('.'))
    override val callableId: CallableId?
        get() = lintLibraryCallableId(fqName, call?.ownerClassId ?: sym?.owner?.takeIf { it.isNotEmpty() }?.let { KcLint.binaryClassId(it).asString() }.orEmpty())
    override val returnType: KaType get() = session.type(call?.declaredReturnType ?: call?.returnType ?: sym?.type)
    override val valueParameters: List<KaValueParameterSymbol> by lazy(LazyThreadSafetyMode.PUBLICATION) {
        call?.params.orEmpty().map { p ->
            KcLintLibraryValueParameterSymbol(session, Name.guessByFirstCharacter(p.name.ifEmpty { "p" }),
                if (p.vararg) lintVarargElement(session, p.type) else session.type(p.type), p.vararg, p.crossinline, p.noinline, call?.fromJava == true)
        }
    }
    override val typeParameters: List<KaTypeParameterSymbol>
        get() = call?.typeParameterNames.orEmpty().map { KcLibraryTypeParameterSymbol(session.round, Name.identifier(it)) }
    override val receiverParameter: KaReceiverParameterSymbol?
        get() = call?.declaredReceiver?.let { KcLintReceiverParameterSymbol(this, session.type(it), null) }
    override val isExtension: Boolean get() = call?.extension == true
    override val isSuspend: Boolean get() = call?.suspend == true
    override val isInline: Boolean get() = call?.inline == true
    override val isOperator: Boolean get() = call?.operator == true
    override val isInfix: Boolean get() = KcMissing.miss("library KaNamedFunctionSymbol.isInfix", false)
    override val isOverride: Boolean get() = false
    override val isTailRec get() = false
    override val isExternal get() = false
    override val isExpect get() = false
    override val isActual get() = false
    override val isStatic: Boolean get() = false
    override val isBuiltinFunctionInvoke: Boolean get() = call?.valueInvocation == true
    override val hasStableParameterNames: Boolean get() = call?.stableNames ?: true
    override val modality get() = KaSymbolModality.FINAL
    override val visibility get() = KaSymbolVisibility.PUBLIC
    override val compilerVisibility: Visibility get() = Visibilities.Public
    override val location get() = if (call?.ownerClassId.isNullOrEmpty()) KaSymbolLocation.TOP_LEVEL else KaSymbolLocation.CLASS
    override val contextParameters: List<KaContextParameterSymbol> get() = emptyList()
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override val annotations: KaAnnotationList get() = KcLintAnnotations.ofNames(session, call?.annotations.orEmpty())
    override fun createPointer(): KaSymbolPointer<KaNamedFunctionSymbol> = KcLintPointer(this)
    override fun toString() = "fun $fqName"
}

internal class KcLintLibraryConstructorSymbol(private val session: KcLint, private val call: KcFCall, private val cls: KtClassOrObject?) : StubKaConstructorSymbol() {
    override val token get() = session.token
    override val psi: PsiElement? get() = null
    override val origin get() = if (cls != null) KaSymbolOrigin.SOURCE else lintLibraryOrigin(call.fromJava)
    override val containingClassId: ClassId?
        get() = cls?.getClassId() ?: call.ownerClassId.takeIf { it.isNotEmpty() }?.let { ClassId.fromString(it) }
    override val returnType: KaType get() = session.type(call.declaredReturnType ?: call.returnType)
    override val valueParameters: List<KaValueParameterSymbol> by lazy(LazyThreadSafetyMode.PUBLICATION) {
        call.params.map { p ->
            KcLintLibraryValueParameterSymbol(session, Name.guessByFirstCharacter(p.name.ifEmpty { "p" }),
                if (p.vararg) lintVarargElement(session, p.type) else session.type(p.type), p.vararg, p.crossinline, p.noinline, call.fromJava)
        }
    }
    override val typeParameters: List<KaTypeParameterSymbol> get() = emptyList()
    override val isPrimary get() = cls != null
    override val hasStableParameterNames get() = call.stableNames
    override val isExpect get() = false
    override val isActual get() = false
    override val isExternal get() = false
    override val visibility get() = KaSymbolVisibility.PUBLIC
    override val compilerVisibility: Visibility get() = Visibilities.Public
    override val annotations: KaAnnotationList get() = KcLintAnnotations.ofNames(session, call.annotations)
    override fun createPointer(): KaSymbolPointer<KaConstructorSymbol> = KcLintPointer(this)
}

internal class KcLintLibraryPropertySymbol(private val session: KcLint, private val sym: KcFSym?, private val getterCall: KcFCall?) : StubKaKotlinPropertySymbol() {
    override val token get() = session.token
    override val psi: PsiElement? get() = null
    override val origin get() = lintLibraryOrigin(getterCall?.fromJava == true)
    override val name: Name
        get() = Name.guessByFirstCharacter(sym?.name ?: getterCall!!.name.substringAfterLast("<get-").removeSuffix(">"))
    override val callableId: CallableId?
        get() {
            val owner = getterCall?.ownerClassId ?: sym?.owner?.takeIf { it.isNotEmpty() }?.let { KcLint.binaryClassId(it).asString() }.orEmpty()
            val fq = sym?.qualifiedName?.takeIf { it.isNotEmpty() }
                ?: getterCall?.let { it.name.substringBeforeLast('.') + "." + name.asString() }
                ?: (owner.replace('/', '.').takeIf { it.isNotEmpty() }?.let { "$it." } ?: "") + name.asString()
            return lintLibraryCallableId(fq, owner)
        }
    override val returnType: KaType get() = session.type(sym?.type ?: getterCall?.declaredReturnType)
    override val isVal: Boolean get() = KcMissing.miss("library KaKotlinPropertySymbol.isVal", true)
    override val hasGetter get() = true
    override val hasSetter get() = !isVal
    override val getter: KaPropertyGetterSymbol? get() = null
    override val setter: KaPropertySetterSymbol? get() = null
    override val hasBackingField get() = KcMissing.miss("library KaKotlinPropertySymbol.hasBackingField", false)
    override val backingFieldSymbol: KaBackingFieldSymbol? get() = null
    override val receiverParameter: KaReceiverParameterSymbol? get() = null
    override val isExtension get() = getterCall?.extension == true
    override val typeParameters: List<KaTypeParameterSymbol> get() = emptyList()
    override val isConst get() = false
    override val isLateInit get() = false
    override val isOverride get() = false
    override val isExternal get() = false
    override val isExpect get() = false
    override val isActual get() = false
    override val isStatic get() = false
    override val isFromPrimaryConstructor get() = false
    override val initializer: org.jetbrains.kotlin.analysis.api.KaInitializerValue? get() = null
    override val modality get() = KaSymbolModality.FINAL
    override val visibility get() = KaSymbolVisibility.PUBLIC
    override val compilerVisibility: Visibility get() = Visibilities.Public
    override val location get() = if (callableId?.classId == null) KaSymbolLocation.TOP_LEVEL else KaSymbolLocation.CLASS
    override val contextParameters: List<KaContextParameterSymbol> get() = emptyList()
    override val contextReceivers: List<KaContextReceiver> get() = emptyList()
    override val annotations: KaAnnotationList get() = KcLintAnnotations.ofNames(session, getterCall?.annotations.orEmpty())
    override fun createPointer(): KaSymbolPointer<KaKotlinPropertySymbol> = KcLintPointer(this)
    override fun toString() = "val $callableId"
}

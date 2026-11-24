package io.github.barteo.kc.aa

import com.intellij.psi.PsiElement
import com.intellij.psi.PsiReference
import com.intellij.psi.ResolveResult
import com.intellij.psi.impl.source.resolve.ResolveCache
import org.jetbrains.kotlin.idea.references.KtArrayAccessReference
import org.jetbrains.kotlin.idea.references.KtCollectionLiteralReference
import org.jetbrains.kotlin.idea.references.KtConstructorDelegationReference
import org.jetbrains.kotlin.idea.references.KtDestructuringDeclarationReference
import org.jetbrains.kotlin.idea.references.KtForLoopInReference
import org.jetbrains.kotlin.idea.references.KtInvokeFunctionReference
import org.jetbrains.kotlin.idea.references.KtPropertyDelegationMethodsReference
import org.jetbrains.kotlin.idea.references.KtReference
import org.jetbrains.kotlin.idea.references.KtSimpleNameReference
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.psi.KotlinReferenceProvidersService
import org.jetbrains.kotlin.psi.KtArrayAccessExpression
import org.jetbrains.kotlin.psi.KtCallExpression
import org.jetbrains.kotlin.psi.KtCollectionLiteralExpression
import org.jetbrains.kotlin.psi.KtConstructorDelegationReferenceExpression
import org.jetbrains.kotlin.psi.KtDestructuringDeclarationEntry
import org.jetbrains.kotlin.psi.KtForExpression
import org.jetbrains.kotlin.psi.KtImplementationDetail
import org.jetbrains.kotlin.psi.KtImportAlias
import org.jetbrains.kotlin.psi.KtPropertyDelegate
import org.jetbrains.kotlin.psi.KtSimpleNameExpression

/**
 * The references of a linter's PSI ([KcStandalone]), the ones the Analysis
 * API's FIR implementation gives each kind of element. Clients take them as
 * `mainReference` and resolve them with `KaSession.resolveToSymbol`
 * ([KcAnalysis]). kc resolves through the session, not through IntelliJ's
 * references, so none of them resolves to PSI itself.
 */
@OptIn(KtImplementationDetail::class)
internal object KcReferenceProviders : KotlinReferenceProvidersService() {
    override fun getReferences(psiElement: PsiElement): Array<PsiReference> = when (psiElement) {
        is KtSimpleNameExpression -> arrayOf(KcSimpleNameReference(psiElement))
        is KtCallExpression -> arrayOf(KcInvokeFunctionReference(psiElement))
        is KtArrayAccessExpression -> arrayOf(KcArrayAccessReference(psiElement))
        is KtForExpression -> arrayOf(KcForLoopInReference(psiElement))
        is KtPropertyDelegate -> arrayOf(KcPropertyDelegationMethodsReference(psiElement))
        is KtDestructuringDeclarationEntry -> arrayOf(KcDestructuringDeclarationReference(psiElement))
        is KtConstructorDelegationReferenceExpression -> arrayOf(KcConstructorDelegationReference(psiElement))
        is KtCollectionLiteralExpression -> arrayOf(KcCollectionLiteralReference(psiElement))
        else -> PsiReference.EMPTY_ARRAY
    }
}

private object KcNoResolver : ResolveCache.PolyVariantResolver<KtReference> {
    override fun resolve(ref: KtReference, incompleteCode: Boolean): Array<ResolveResult> = ResolveResult.EMPTY_ARRAY
}

@OptIn(KtImplementationDetail::class)
internal class KcSimpleNameReference(expression: KtSimpleNameExpression) : KtSimpleNameReference(expression) {
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = KcNoResolver
    override val resolvesByNames: Collection<Name> get() = listOf(expression.getReferencedNameAsName())
    override fun isReferenceToViaExtension(element: PsiElement): Boolean = false
    override fun getImportAlias(): KtImportAlias? = null
}

@OptIn(KtImplementationDetail::class)
internal class KcInvokeFunctionReference(e: KtCallExpression) : KtInvokeFunctionReference(e) {
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = KcNoResolver
}

@OptIn(KtImplementationDetail::class)
internal class KcArrayAccessReference(e: KtArrayAccessExpression) : KtArrayAccessReference(e) {
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = KcNoResolver
}

@OptIn(KtImplementationDetail::class)
internal class KcForLoopInReference(e: KtForExpression) : KtForLoopInReference(e) {
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = KcNoResolver
}

@OptIn(KtImplementationDetail::class)
internal class KcPropertyDelegationMethodsReference(e: KtPropertyDelegate) : KtPropertyDelegationMethodsReference(e) {
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = KcNoResolver
}

@OptIn(KtImplementationDetail::class)
internal class KcDestructuringDeclarationReference(e: KtDestructuringDeclarationEntry) : KtDestructuringDeclarationReference(e) {
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = KcNoResolver
}

@OptIn(KtImplementationDetail::class)
internal class KcConstructorDelegationReference(e: KtConstructorDelegationReferenceExpression) : KtConstructorDelegationReference(e) {
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = KcNoResolver
}

@OptIn(KtImplementationDetail::class)
internal class KcCollectionLiteralReference(e: KtCollectionLiteralExpression) : KtCollectionLiteralReference(e) {
    override val resolver: ResolveCache.PolyVariantResolver<KtReference> get() = KcNoResolver
}

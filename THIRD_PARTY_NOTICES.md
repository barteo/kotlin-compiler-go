# Third-party notices

## Kotlin and IntelliJ Platform

Parts of package `psi` are Go translations of source code from the Kotlin
compiler 2.4.21 (https://github.com/JetBrains/kotlin) and the IntelliJ
Platform (https://github.com/JetBrains/intellij-community), both
Copyright JetBrains s.r.o. and contributors, licensed under the Apache
License, Version 2.0 (see [LICENSE](LICENSE)). The Kotlin sources are
under `compiler/psi/parser/src`, `compiler/psi/psi-api/src` and
`compiler/psi/psi-impl/src` in the Kotlin repository:

| Go file | Translated from |
|---|---|
| `psi/lexer.go`, `psi/tokens.go` | `org.jetbrains.kotlin.lexer` (`Kotlin.flex`, `KtTokens`) |
| `psi/builder.go` | `com.intellij.lang.impl.PsiBuilderImpl`, `org.jetbrains.kotlin.parsing.SemanticWhitespaceAwarePsiBuilderImpl`, `TruncatedSemanticWhitespaceAwarePsiBuilder` |
| `psi/tree.go` | `PsiBuilderImpl` (whitespace balancing, tree building), `org.jetbrains.kotlin.parsing.KotlinWhitespaceAndCommentsBinders`, `com.intellij.lang.WhitespacesBinders` |
| `psi/parsing_base.go` | `org.jetbrains.kotlin.parsing.AbstractKotlinParsing` |
| `psi/parsing_kotlin.go`, `psi/parsing_shared.go` | `org.jetbrains.kotlin.parsing.KotlinParsing` |
| `psi/parsing_expr.go` | `org.jetbrains.kotlin.parsing.KotlinExpressionParsing`, `org.jetbrains.kotlin.lang.BinaryOperationPrecedence`, `org.jetbrains.kotlin.psi.stubs.elements.KtTokenSets` |
| `psi/kinds.txt` | element and token type names from `KtNodeTypes`, `KtTokens`, `KDocTokens` |

The translations were changed to fit Go and this package's data
structures; see the comments in each file.

# inject-jars.sh lists kotlin-inject's and kotlin-inject-anvil's jars as
# GROUP:ARTIFACT:VERSION, for build.sh: the class paths of
# kotlin-inject-compiler-ksp 0.9.0 and kotlin-inject-anvil's compiler
# 0.1.7, but for KSP's API and the Kotlin standard library, which KSP
# provides.
#
# inject_libs is the module's libraries (with KSP's Kotlin standard
# library before them), and inject_procs the processors' class path.
inject_libs=(me.tatarka.inject:kotlin-inject-runtime-jvm:0.9.0 software.amazon.lastmile.kotlin.inject.anvil:runtime-jvm:0.1.7
	software.amazon.lastmile.kotlin.inject.anvil:runtime-optional-jvm:0.1.7)
inject_procs=(me.tatarka.inject:kotlin-inject-compiler-ksp:0.9.0 me.tatarka.inject:kotlin-inject-compiler-core:0.9.0
	me.tatarka.inject:ast-ksp:0.9.0 me.tatarka.inject:ast-core:0.9.0 me.tatarka.inject:kotlin-inject-runtime-kmp-jvm:0.9.0
	me.tatarka.inject:kotlin-inject-runtime-jvm:0.9.0 com.squareup:kotlinpoet-jvm:2.2.0 com.squareup:kotlinpoet-ksp:2.2.0
	software.amazon.lastmile.kotlin.inject.anvil:compiler:0.1.7 software.amazon.lastmile.kotlin.inject.anvil:compiler-utils:0.1.7
	software.amazon.lastmile.kotlin.inject.anvil:runtime-jvm:0.1.7)

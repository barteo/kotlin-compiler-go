# hilt-jars.sh lists Hilt's jars as GROUP:ARTIFACT:VERSION, for build.sh
# (which fetches them) and bench/ksp/hilt/run.sh (which times Hilt's
# processors): the class paths Gradle resolves for hilt-compiler 2.59.2,
# with Dagger's, but for KSP's API and the Kotlin standard library, which
# KSP provides.
#
# hilt_libs is the module's libraries (with KSP's Kotlin standard library
# before them), and hilt_procs the processors' class path.
hilt_libs=(com.google.dagger:hilt-core:2.59.2 com.google.dagger:dagger:2.59.2
	com.google.code.findbugs:jsr305:3.0.2 jakarta.inject:jakarta.inject-api:2.0.1 javax.inject:javax.inject:1 org.jspecify:jspecify:1.0.0)
hilt_procs=(com.google.dagger:hilt-compiler:2.59.2 com.google.dagger:dagger-compiler:2.59.2 com.google.dagger:dagger-spi:2.59.2
	com.google.dagger:dagger:2.59.2 com.google.googlejavaformat:google-java-format:1.33.0 com.google.guava:guava:33.0.0-jre
	com.google.code.findbugs:jsr305:3.0.2 com.google.guava:failureaccess:1.0.2 com.squareup:javapoet:1.13.0
	javax.inject:javax.inject:1 net.ltgt.gradle.incap:incap:0.2 org.jetbrains.kotlin:kotlin-metadata-jvm:2.2.20
	com.squareup:kotlinpoet:1.11.0 org.jetbrains.kotlin:kotlin-reflect:1.6.10 org.jetbrains.kotlin:kotlin-stdlib-jdk8:1.8.0
	org.jetbrains.kotlin:kotlin-stdlib-jdk7:1.8.0 jakarta.inject:jakarta.inject-api:2.0.1 org.jspecify:jspecify:1.0.0
	org.checkerframework:checker-compat-qual:2.5.3 org.checkerframework:checker-qual:3.41.0
	com.google.errorprone:error_prone_annotations:2.23.0 org.jetbrains:annotations:13.0)

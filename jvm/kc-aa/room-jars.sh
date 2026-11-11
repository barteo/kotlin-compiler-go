# room-jars.sh lists Room's jars as GROUP:ARTIFACT:VERSION, for build.sh
# (which fetches them) and bench/ksp/room/run.sh (which times Room's
# processor): the class paths Gradle resolves for room-compiler 2.8.4, but
# for KSP's API and the Kotlin standard library, which KSP provides.
#
# room_libs is the module's libraries (with KSP's Kotlin standard library
# before them); room_procs the processor's class path; and
# room_serialization the kotlinx-serialization stock KSP needs beside it.
# On kc it is the one the KSP fork already has.
room_libs=(androidx.room:room-runtime-jvm:2.8.4 androidx.room:room-common-jvm:2.8.4
	androidx.collection:collection-jvm:1.5.0 androidx.sqlite:sqlite-jvm:2.6.2 androidx.annotation:annotation-jvm:1.9.1
	org.jetbrains.kotlinx:kotlinx-coroutines-core-jvm:1.8.1 org.jetbrains:annotations:23.0.0)
room_procs=(androidx.room:room-migration-jvm:2.8.4 androidx.room:room-common-jvm:2.8.4 androidx.room:room-external-antlr:2.8.4
	androidx.room:room-compiler-processing:2.8.4 androidx.room:room-compiler:2.8.4 com.google.auto:auto-common:1.2.1
	com.google.auto.value:auto-value-annotations:1.6.3 com.intellij:annotations:12.0 com.squareup:kotlinpoet-javapoet:2.0.0
	com.squareup:javapoet:1.13.0 commons-codec:commons-codec:1.15 org.jetbrains.kotlin:kotlin-metadata-jvm:2.2.0
	com.squareup:kotlinpoet-jvm:2.0.0 org.jetbrains.kotlin:kotlin-reflect:2.0.10 androidx.annotation:annotation-jvm:1.9.1
	org.xerial:sqlite-jdbc:3.41.2.2 com.google.guava:guava:33.2.1-jre org.jspecify:jspecify:1.0.0 org.jetbrains:annotations:13.0
	com.google.guava:failureaccess:1.0.2 com.google.code.findbugs:jsr305:3.0.2 org.checkerframework:checker-qual:3.42.0
	com.google.errorprone:error_prone_annotations:2.26.1)
room_serialization=(org.jetbrains.kotlinx:kotlinx-serialization-core-jvm:1.8.1 org.jetbrains.kotlinx:kotlinx-serialization-json-jvm:1.8.1)

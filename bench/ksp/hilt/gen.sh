#!/usr/bin/env bash
# gen.sh writes a Hilt module of N files into DIR/h, for timing Hilt's
# processors (with Dagger's) on more than test/hilt's one file:
#
#	bench/ksp/hilt/gen.sh DIR [N]
#
# N (default 100) is the number of FeatureNNN.kt files, each with @Inject
# classes (one @Singleton, one depending on the previous file's), a
# @Binds module and an object module with @Named, @IntoSet and @IntoMap
# bindings, all @InstallIn(SingletonComponent::class), an @EntryPoint,
# and an @AssistedInject class with its @AssistedFactory. Every tenth file
# also defines a scoped child component (@DefineComponent, with a builder,
# a module and an entry point installed in it). There is no Hilt root
# (@HiltAndroidApp is Android's), so Hilt generates factories and its
# aggregated dependencies, not components. The output only depends on N.
set -euo pipefail

[ $# -ge 1 ] || { sed -n '2,15p' "$0" >&2; exit 2; }
dir=$1 n=${2:-100}
rm -rf "$dir"
mkdir -p "$dir/h"

for ((i = 0; i < n; i++)); do
	F=$(printf 'Feature%03d' "$i")
	if ((i > 0)); then
		P=$(printf 'Feature%03d' $((i - 1)))
		prev=", private val prev: ${P}Clock"
	else
		prev=""
	fi
	{
		cat <<EOF
package h

import dagger.Binds
import dagger.Module
import dagger.Provides
import dagger.assisted.Assisted
import dagger.assisted.AssistedFactory
import dagger.assisted.AssistedInject
import dagger.hilt.DefineComponent
import dagger.hilt.EntryPoint
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import dagger.multibindings.IntoMap
import dagger.multibindings.IntoSet
import dagger.multibindings.StringKey
import javax.inject.Inject
import javax.inject.Named
import javax.inject.Scope
import javax.inject.Singleton

interface ${F}Repo {
    fun load(id: Long): String
}

class ${F}Clock @Inject constructor() {
    fun now(): Long = $i
}

class ${F}Api @Inject constructor(private val clock: ${F}Clock$prev) {
    fun call(id: Long): String = id.toString() + "@" + clock.now()
}

@Singleton
class ${F}RepoImpl @Inject constructor(private val api: ${F}Api, @Named("base$i") private val base: String) : ${F}Repo {
    override fun load(id: Long): String = base + api.call(id)
}

class ${F}UseCase @Inject constructor(private val repo: ${F}Repo, private val plugins: Set<String>) {
    fun run(id: Long): String = repo.load(id) + plugins.size
}

class ${F}Job @AssistedInject constructor(@Assisted val id: Long, private val repo: ${F}Repo) {
    fun run(): String = repo.load(id)
}

@AssistedFactory
interface ${F}JobFactory {
    fun create(id: Long): ${F}Job
}

@Module
@InstallIn(SingletonComponent::class)
interface ${F}RepoModule {
    @Binds
    fun repo(impl: ${F}RepoImpl): ${F}Repo
}

@Module
@InstallIn(SingletonComponent::class)
object ${F}NetModule {
    @Provides
    @Named("base$i")
    fun base(): String = "http://$i/"

    @Provides
    @IntoSet
    fun plugin(): String = "p$i"

    @Provides
    @IntoMap
    @StringKey("k$i")
    fun entry(clock: ${F}Clock): Long = clock.now()
}

@EntryPoint
@InstallIn(SingletonComponent::class)
interface ${F}EntryPoint {
    fun useCase(): ${F}UseCase

    fun jobs(): ${F}JobFactory
}
EOF
		if ((i % 10 == 0)); then
			cat <<EOF

@Scope
@Retention(AnnotationRetention.RUNTIME)
annotation class ${F}Scoped

@${F}Scoped
@DefineComponent(parent = SingletonComponent::class)
interface ${F}Component

@DefineComponent.Builder
interface ${F}ComponentBuilder {
    fun build(): ${F}Component
}

class ${F}Session @Inject constructor(val repo: ${F}Repo)

@Module
@InstallIn(${F}Component::class)
object ${F}SessionModule {
    @Provides
    @${F}Scoped
    fun id(): Long = $i
}

@EntryPoint
@InstallIn(${F}Component::class)
interface ${F}SessionEntryPoint {
    fun session(): ${F}Session

    fun id(): Long
}
EOF
		fi
	} > "$dir/h/$F.kt"
done

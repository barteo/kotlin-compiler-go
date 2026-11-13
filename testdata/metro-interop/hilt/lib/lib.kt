package lib

import dagger.Module
import dagger.Provides
import dagger.hilt.DefineComponent
import dagger.hilt.EntryPoint
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import javax.inject.Scope
import javax.inject.Singleton

class Counter {
    var n = 0
}

class Feature(val name: String)

class Marker(val name: String)

// Installed in SingletonComponent, which Metro maps to javax's
// @Singleton: a graph with that scope includes it.
@Module
@InstallIn(SingletonComponent::class)
object LibModule {
    @Provides
    @Singleton
    fun counter(): Counter = Counter()

    @Provides
    fun greeting(): String = "hello from lib"
}

@EntryPoint
@InstallIn(SingletonComponent::class)
interface LibEntryPoint {
    fun counter(): Counter

    fun greeting(): String
}

// A custom component maps to its scope annotation.
@Scope
annotation class FeatureScoped

@FeatureScoped
@DefineComponent(parent = SingletonComponent::class)
interface FeatureComponent

@Module
@InstallIn(FeatureComponent::class)
object FeatureModule {
    @Provides
    @FeatureScoped
    fun feature(): Feature = Feature("feature")
}

// Contributed only by the hilt_aggregated_deps marker, as Hilt's
// processor writes them.
@Module
object MarkerModule {
    @Provides
    fun marker(): Marker = Marker("marker")
}

interface MarkerEntryPoint {
    fun marker(): Marker
}

// A test's marker, which Metro skips: its module would bind Marker twice.
@Module
object TestMarkerModule {
    @Provides
    fun marker(): Marker = Marker("test")
}

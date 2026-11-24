package hilt_aggregated_deps

import dagger.hilt.processor.internal.aggregateddeps.AggregatedDeps

@AggregatedDeps(
    components = ["dagger.hilt.components.SingletonComponent"],
    modules = ["lib.MarkerModule"],
    entryPoints = ["lib.MarkerEntryPoint"],
)
class _lib_MarkerModule

@AggregatedDeps(
    components = ["dagger.hilt.components.SingletonComponent"],
    test = "lib.SomeTest",
    modules = ["lib.TestMarkerModule"],
)
class _lib_TestMarkerModule

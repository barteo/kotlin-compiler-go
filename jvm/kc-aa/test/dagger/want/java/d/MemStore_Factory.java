package d;

import dagger.Lazy;
import dagger.internal.DaggerGenerated;
import dagger.internal.DoubleCheck;
import dagger.internal.Factory;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;

@ScopeMetadata
@QualifierMetadata
@DaggerGenerated
@Generated(
    value = "dagger.internal.codegen.ComponentProcessor",
    comments = "https://dagger.dev"
)
@SuppressWarnings({
    "unchecked",
    "rawtypes",
    "KotlinInternal",
    "KotlinInternalInJava",
    "cast",
    "deprecation",
    "nullness:initialization.field.uninitialized"
})
public final class MemStore_Factory implements Factory<MemStore> {
  private final Provider<Repo> repoProvider;

  private final Provider<Clock> clocksProvider;

  private MemStore_Factory(Provider<Repo> repoProvider, Provider<Clock> clocksProvider) {
    this.repoProvider = repoProvider;
    this.clocksProvider = clocksProvider;
  }

  @Override
  public MemStore get() {
    return newInstance(DoubleCheck.lazy(repoProvider), clocksProvider);
  }

  public static MemStore_Factory create(Provider<Repo> repoProvider,
      Provider<Clock> clocksProvider) {
    return new MemStore_Factory(repoProvider, clocksProvider);
  }

  public static MemStore newInstance(Lazy<Repo> repo, javax.inject.Provider<Clock> clocks) {
    return new MemStore(repo, clocks);
  }
}

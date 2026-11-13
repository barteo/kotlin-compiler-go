package d;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Provider;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;

@ScopeMetadata("javax.inject.Singleton")
@QualifierMetadata("javax.inject.Named")
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
public final class Repo_Factory implements Factory<Repo> {
  private final Provider<Clock> clockProvider;

  private final Provider<String> urlProvider;

  private final Provider<Integer> portProvider;

  private Repo_Factory(Provider<Clock> clockProvider, Provider<String> urlProvider,
      Provider<Integer> portProvider) {
    this.clockProvider = clockProvider;
    this.urlProvider = urlProvider;
    this.portProvider = portProvider;
  }

  @Override
  public Repo get() {
    return newInstance(clockProvider.get(), urlProvider.get(), portProvider.get());
  }

  public static Repo_Factory create(Provider<Clock> clockProvider, Provider<String> urlProvider,
      Provider<Integer> portProvider) {
    return new Repo_Factory(clockProvider, urlProvider, portProvider);
  }

  public static Repo newInstance(Clock clock, String url, int port) {
    return new Repo(clock, url, port);
  }
}

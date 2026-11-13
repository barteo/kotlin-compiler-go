package h;

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
public final class MemStore_Factory implements Factory<MemStore> {
  private final Provider<String> urlProvider;

  private MemStore_Factory(Provider<String> urlProvider) {
    this.urlProvider = urlProvider;
  }

  @Override
  public MemStore get() {
    return newInstance(urlProvider.get());
  }

  public static MemStore_Factory create(Provider<String> urlProvider) {
    return new MemStore_Factory(urlProvider);
  }

  public static MemStore newInstance(String url) {
    return new MemStore(url);
  }
}

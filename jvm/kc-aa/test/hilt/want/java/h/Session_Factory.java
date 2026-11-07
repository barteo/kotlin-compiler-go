package h;

import dagger.internal.DaggerGenerated;
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
public final class Session_Factory implements Factory<Session> {
  private final Provider<Store> storeProvider;

  private Session_Factory(Provider<Store> storeProvider) {
    this.storeProvider = storeProvider;
  }

  @Override
  public Session get() {
    return newInstance(storeProvider.get());
  }

  public static Session_Factory create(Provider<Store> storeProvider) {
    return new Session_Factory(storeProvider);
  }

  public static Session newInstance(Store store) {
    return new Session(store);
  }
}

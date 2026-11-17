package a;

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
public final class ScreenModel_Factory implements Factory<ScreenModel> {
  private final Provider<Bar> barProvider;

  private ScreenModel_Factory(Provider<Bar> barProvider) {
    this.barProvider = barProvider;
  }

  @Override
  public ScreenModel get() {
    return newInstance(barProvider.get());
  }

  public static ScreenModel_Factory create(Provider<Bar> barProvider) {
    return new ScreenModel_Factory(barProvider);
  }

  public static ScreenModel newInstance(Bar bar) {
    return new ScreenModel(bar);
  }
}

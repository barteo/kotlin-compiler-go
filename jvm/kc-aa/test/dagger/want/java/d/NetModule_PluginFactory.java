package d;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Preconditions;
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
public final class NetModule_PluginFactory implements Factory<String> {
  private final Provider<Clock> clockProvider;

  private NetModule_PluginFactory(Provider<Clock> clockProvider) {
    this.clockProvider = clockProvider;
  }

  @Override
  public String get() {
    return plugin(clockProvider.get());
  }

  public static NetModule_PluginFactory create(Provider<Clock> clockProvider) {
    return new NetModule_PluginFactory(clockProvider);
  }

  public static String plugin(Clock clock) {
    return Preconditions.checkNotNullFromProvides(NetModule.INSTANCE.plugin(clock));
  }
}

package h;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Preconditions;
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
  @Override
  public String get() {
    return plugin();
  }

  public static NetModule_PluginFactory create() {
    return InstanceHolder.INSTANCE;
  }

  public static String plugin() {
    return Preconditions.checkNotNullFromProvides(NetModule.INSTANCE.plugin());
  }

  private static final class InstanceHolder {
    static final NetModule_PluginFactory INSTANCE = new NetModule_PluginFactory();
  }
}

package h;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
import dagger.internal.Preconditions;
import dagger.internal.QualifierMetadata;
import dagger.internal.ScopeMetadata;
import javax.annotation.processing.Generated;

@ScopeMetadata
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
public final class NetModule_UrlFactory implements Factory<String> {
  @Override
  public String get() {
    return url();
  }

  public static NetModule_UrlFactory create() {
    return InstanceHolder.INSTANCE;
  }

  public static String url() {
    return Preconditions.checkNotNullFromProvides(NetModule.INSTANCE.url());
  }

  private static final class InstanceHolder {
    static final NetModule_UrlFactory INSTANCE = new NetModule_UrlFactory();
  }
}

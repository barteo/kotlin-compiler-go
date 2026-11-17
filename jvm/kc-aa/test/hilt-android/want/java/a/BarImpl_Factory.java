package a;

import dagger.internal.DaggerGenerated;
import dagger.internal.Factory;
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
public final class BarImpl_Factory implements Factory<BarImpl> {
  @Override
  public BarImpl get() {
    return newInstance();
  }

  public static BarImpl_Factory create() {
    return InstanceHolder.INSTANCE;
  }

  public static BarImpl newInstance() {
    return new BarImpl();
  }

  private static final class InstanceHolder {
    static final BarImpl_Factory INSTANCE = new BarImpl_Factory();
  }
}

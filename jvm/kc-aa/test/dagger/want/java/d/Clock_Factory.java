package d;

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
public final class Clock_Factory implements Factory<Clock> {
  @Override
  public Clock get() {
    return newInstance();
  }

  public static Clock_Factory create() {
    return InstanceHolder.INSTANCE;
  }

  public static Clock newInstance() {
    return new Clock();
  }

  private static final class InstanceHolder {
    static final Clock_Factory INSTANCE = new Clock_Factory();
  }
}

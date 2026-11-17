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
public final class Foo_Factory implements Factory<Foo> {
  @Override
  public Foo get() {
    return newInstance();
  }

  public static Foo_Factory create() {
    return InstanceHolder.INSTANCE;
  }

  public static Foo newInstance() {
    return new Foo();
  }

  private static final class InstanceHolder {
    static final Foo_Factory INSTANCE = new Foo_Factory();
  }
}

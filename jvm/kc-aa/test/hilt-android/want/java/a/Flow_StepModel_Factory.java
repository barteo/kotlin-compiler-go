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
public final class Flow_StepModel_Factory implements Factory<Flow.StepModel> {
  @Override
  public Flow.StepModel get() {
    return newInstance();
  }

  public static Flow_StepModel_Factory create() {
    return InstanceHolder.INSTANCE;
  }

  public static Flow.StepModel newInstance() {
    return new Flow.StepModel();
  }

  private static final class InstanceHolder {
    static final Flow_StepModel_Factory INSTANCE = new Flow_StepModel_Factory();
  }
}

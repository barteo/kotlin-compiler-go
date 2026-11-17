import java.lang.annotation.*;

public class Targets {
    @Target(ElementType.TYPE)
    public @interface type {}

    @Target({ElementType.METHOD, ElementType.FIELD})
    public @interface member {}

    @Target(ElementType.PARAMETER)
    @kotlin.annotation.Target(allowedTargets = kotlin.annotation.AnnotationTarget.FUNCTION)
    public @interface paramOrFun {}

    @kotlin.annotation.Target(allowedTargets = kotlin.annotation.AnnotationTarget.TYPE)
    public @interface kotlinType {}
}

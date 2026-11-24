import java.lang.annotation.*;

@Retention(RetentionPolicy.RUNTIME)
public @interface MyAnn {
    Class<?>[] value();
}

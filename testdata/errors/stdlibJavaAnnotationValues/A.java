public @interface A {
    String[] value();
    Class<?> x() default Integer.class;
    int y();
}

package app;

import java.util.ArrayList;
import java.util.List;

/** A Java subclass of a Kotlin class, which the Kotlin code uses. */
public class Square extends Shape implements Named {
    public static final int SIDES = 4;
    public static final int CORNERS = SIDES * 2 - (1 << 2);
    public static final String KIND = "sq" + "uare";
    public static final long BIG = 1L << 40;
    public static final char FIRST = 'S', LAST = (char) ('e' - 32);
    public static final double HALF = 1 / 2.0;
    public static final boolean DEBUG = Boolean.parseBoolean("true");

    public final double side;
    public Shape twin;

    public Square(double side) {
        super("square");
        this.side = side;
    }

    @Override
    public double area() {
        return side * side;
    }

    @Override
    public String label() {
        return KIND + ":" + side;
    }

    public static <T> List<T> twice(T x) {
        List<T> l = new ArrayList<>();
        l.add(x);
        l.add(x);
        return l;
    }

    public static int sum(int... xs) {
        int s = 0;
        for (int x : xs) {
            s += x;
        }
        return s;
    }

    public static String describe(Shape s) {
        return "shape " + s.getName();
    }

    public static class Builder {
        private double side;

        public Builder side(double s) {
            side = s;
            return this;
        }

        public Square build() {
            return new Square(side);
        }
    }

    public enum Color {
        RED,
        GREEN {
            @Override
            public String code() {
                return "g";
            }
        };

        public String code() {
            return name().toLowerCase();
        }
    }

    public interface Visitor<R> {
        R visit(Square s);

        default String name() {
            return "visitor";
        }
    }

    public <R> R accept(Visitor<R> v) {
        return v.visit(this);
    }
}

package lib;

import kotlinx.serialization.Serializable;

// A Java class of another module that names its serializer.
@Serializable(with = WeightSerializer.class)
public class Weight {
    public final int grams;

    public Weight(int grams) {
        this.grams = grams;
    }

    @Override
    public boolean equals(Object o) {
        return o instanceof Weight && ((Weight) o).grams == grams;
    }

    @Override
    public int hashCode() {
        return grams;
    }

    @Override
    public String toString() {
        return "Weight(" + grams + ")";
    }
}

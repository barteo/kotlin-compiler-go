package android.text;

import android.os.Parcel;
import android.os.Parcelable;

// Android's TextUtils, for the parcelize tests: a CharSequence is written
// as Android writes one that isn't Spanned, a kind and its string.
public class TextUtils {
    public static void writeToParcel(CharSequence cs, Parcel p, int parcelableFlags) {
        p.writeInt(1);
        p.writeString(cs == null ? null : cs.toString());
    }

    public static final Parcelable.Creator<CharSequence> CHAR_SEQUENCE_CREATOR = new Parcelable.Creator<CharSequence>() {
        @Override
        public CharSequence createFromParcel(Parcel p) {
            p.readInt();
            return p.readString();
        }

        @Override
        public CharSequence[] newArray(int size) {
            return new CharSequence[size];
        }
    };
}

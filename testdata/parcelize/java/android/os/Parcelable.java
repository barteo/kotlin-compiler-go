package android.os;

// Android's Parcelable, for the parcelize tests.
public interface Parcelable {
    int describeContents();

    void writeToParcel(Parcel dest, int flags);

    interface Creator<T> {
        T createFromParcel(Parcel source);

        T[] newArray(int size);
    }
}

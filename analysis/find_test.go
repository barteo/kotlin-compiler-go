package analysis

import (
	"testing"

	"github.com/barteo/kotlin-compiler-go/internal/classpath"
)

func TestFind(t *testing.T) {
	jdk, err := FindJDK()
	want, wantErr := classpath.FindJDK()
	if jdk != want || (err == nil) != (wantErr == nil) {
		t.Errorf("FindJDK() = %q, %v; want %q, %v", jdk, err, want, wantErr)
	}
	jar, err := FindStdlib()
	want, wantErr = classpath.FindStdlib()
	if jar != want || (err == nil) != (wantErr == nil) {
		t.Errorf("FindStdlib() = %q, %v; want %q, %v", jar, err, want, wantErr)
	}
}

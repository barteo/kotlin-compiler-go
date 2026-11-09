package analysis

// LibraryStub renders the library's declaration named qualified, a
// top-level class or the top-level functions and properties of a name,
// or a member of either, as Kotlin source without bodies, for an editor
// to show when the library has no sources. It returns the file's path in
// a package's directory, a/b/C.kt, and its text; ok is false if the
// library has no such declaration.
func (s *Session) LibraryStub(qualified string) (path, text string, ok bool, err error) {
	_, lib, release, err := s.library()
	if err != nil {
		return "", "", false, err
	}
	defer release()
	path, text, ok = lib.Stub(qualified)
	return path, text, ok, nil
}

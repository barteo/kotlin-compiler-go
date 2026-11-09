// Package libsrc finds the sources of a library's declarations, for an
// editor's navigation into the library: the source archive of each jar of
// a class path (a -sources.jar next to it, or in Gradle's cache, and the
// JDK's src.zip), and in it the file and the declaration.
//
// An archive is indexed once, lazily, by what its files declare at the
// top level: a quick scan of each Kotlin file's lines that start a
// declaration, and each Java file's path. A declaration is then located
// in the files that declare its top-level name by parsing them.
package libsrc

import (
	"archive/zip"
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/barteo/kotlin-compiler-go/psi"
)

// A Target is a declaration of a library to locate.
type Target struct {
	// Kind is "class", "function", "constructor" or "property".
	Kind string
	// Name is the declaration's fully qualified name, kotlin.Pair or
	// kotlin.collections.List.size; a constructor's is its class's.
	Name string
	// Binary is the JVM binary name of a class, or of a member's class,
	// if known: java/util/Map$Entry.
	Binary string
	// Parameters are a function's value parameters' types' simple names,
	// T or Int, if Params is set, and Varargs which are varargs.
	Parameters []string
	Varargs    []bool
	Params     bool
	// Receiver is an extension's receiver type's simple name.
	Receiver string
}

// A Location is where a declaration is in a source archive.
type Location struct {
	Archive, Entry string
	Text           string
	// Range is the declaration's name, in bytes of Text.
	Range psi.Range
	// Snap and Node are the parse of Text and the declaration, for a
	// Kotlin file; Doc is the declaration of the same name with a
	// documentation comment, Node's or an expect declaration's it
	// implements, or invalid.
	Snap *psi.Snapshot
	Node psi.NodeID
	Doc  psi.NodeRef
}

// A Finder finds declarations in the source archives of a class path.
type Finder struct {
	archives []string

	mu    sync.Mutex
	found map[string]located // Locate's results, by target
}

type located struct {
	loc Location
	ok  bool
}

// maxFound is how many targets' locations a Finder keeps.
const maxFound = 256

// New returns the finder of the sources of classPath's jars and of the
// JDK at jdkHome ("" for $JAVA_HOME's).
func New(classPath []string, jdkHome string) *Finder {
	f := &Finder{}
	seen := map[string]bool{}
	add := func(p string) {
		if p != "" && !seen[p] {
			seen[p] = true
			f.archives = append(f.archives, p)
		}
	}
	for _, jar := range classPath {
		add(SourcesOf(jar))
	}
	if jdkHome == "" {
		jdkHome = os.Getenv("JAVA_HOME")
	}
	if jdkHome != "" {
		if p := filepath.Join(jdkHome, "lib", "src.zip"); exists(p) {
			add(p)
		}
	}
	return f
}

// Archives returns the source archives f searches.
func (f *Finder) Archives() []string { return f.archives }

func exists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// SourcesOf returns the source archive of a jar, or "": X-sources.jar
// next to X.jar, as Maven's repositories and Kotlin's distribution have
// them, or in Gradle's cache, where each file is in a directory of its
// hash: files-2.1/<group>/<name>/<version>/<hash>/<name>-<version>.jar.
func SourcesOf(jar string) string {
	base, ok := strings.CutSuffix(jar, ".jar")
	if !ok {
		return ""
	}
	if p := base + "-sources.jar"; exists(p) {
		return p
	}
	dir := filepath.Dir(jar)
	if filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(dir))))) != "files-2.1" {
		return ""
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(dir), "*", filepath.Base(base)+"-sources.jar"))
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// index is what an archive's files declare.
type index struct {
	// top are the Kotlin files by package and top-level name.
	top map[topKey][]string
	// byName are the Kotlin files by top-level name.
	byName map[string][]string
	// java are the Java files by their class's binary name, a/b/C,
	// without a module's directory (src.zip's java.base/).
	java map[string]string
}

type topKey struct{ pkg, name string }

type cached struct {
	size int64
	mod  time.Time
	idx  *index
	err  error
}

// indexes are the archives' indexes, by path, while the archive is the
// same.
var indexes sync.Map // string -> *cached

var indexMu sync.Mutex

// indexOf returns the index of archive, reading it if it changed.
func indexOf(archive string) (*index, error) {
	st, err := os.Stat(archive)
	if err != nil {
		return nil, err
	}
	if c, ok := indexes.Load(archive); ok {
		if c := c.(*cached); c.size == st.Size() && c.mod.Equal(st.ModTime()) {
			return c.idx, c.err
		}
	}
	indexMu.Lock()
	defer indexMu.Unlock()
	if c, ok := indexes.Load(archive); ok {
		if c := c.(*cached); c.size == st.Size() && c.mod.Equal(st.ModTime()) {
			return c.idx, c.err
		}
	}
	idx, err := readIndex(archive)
	indexes.Store(archive, &cached{st.Size(), st.ModTime(), idx, err})
	return idx, err
}

func readIndex(archive string) (*index, error) {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	idx := &index{top: map[topKey][]string{}, byName: map[string][]string{}, java: map[string]string{}}
	for _, f := range r.File {
		switch {
		case strings.HasSuffix(f.Name, ".java"):
			name := strings.TrimSuffix(f.Name, ".java")
			if i := strings.IndexByte(name, '/'); i > 0 && strings.Contains(name[:i], ".") {
				name = name[i+1:] // a module's directory: java.base/java/lang/String
			}
			idx.java[name] = f.Name
		case strings.HasSuffix(f.Name, ".kt"):
			text, err := readFile(f)
			if err != nil {
				continue
			}
			pkg, names := scan(text)
			for _, n := range names {
				k := topKey{pkg, n}
				if !slices.Contains(idx.top[k], f.Name) {
					idx.top[k] = append(idx.top[k], f.Name)
					idx.byName[n] = append(idx.byName[n], f.Name)
				}
			}
		}
	}
	return idx, nil
}

func readFile(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	return string(b), err
}

// Read returns the text of an archive's entry.
func Read(archive, entry string) (string, error) {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return "", err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == entry {
			return readFile(f)
		}
	}
	return "", os.ErrNotExist
}

// Locate returns where t is declared, in the first archive that has a
// file declaring it.
func (f *Finder) Locate(t Target) (Location, bool) {
	key := fmt.Sprintf("%#v", t)
	f.mu.Lock()
	r, ok := f.found[key]
	f.mu.Unlock()
	if ok {
		return r.loc, r.ok
	}
	r.loc, r.ok = f.locate(t)
	f.mu.Lock()
	if f.found == nil || len(f.found) >= maxFound {
		f.found = map[string]located{}
	}
	f.found[key] = r
	f.mu.Unlock()
	return r.loc, r.ok
}

func (f *Finder) locate(t Target) (Location, bool) {
	for _, a := range f.archives {
		idx, err := indexOf(a)
		if err != nil {
			continue
		}
		if loc, ok := locateKotlin(a, idx, t, func(e string) (string, error) { return Read(a, e) }); ok {
			return loc, true
		}
		if loc, ok := locateJava(a, idx, t); ok {
			return loc, true
		}
	}
	return Location{}, false
}

// LocateIn finds t in text, a Kotlin file at entry of archive, as Locate
// does in an archive: for a stub rendered from the library's metadata.
func LocateIn(archive, entry, text string, t Target) (Location, bool) {
	idx := &index{top: map[topKey][]string{}, byName: map[string][]string{}}
	pkg, names := scan(text)
	for _, n := range names {
		idx.top[topKey{pkg, n}] = []string{entry}
		idx.byName[n] = []string{entry}
	}
	return locateKotlin(archive, idx, t, func(string) (string, error) { return text, nil })
}

// candidate is a declaration that may be the target, and its score.
type candidate struct {
	entry, text string
	snap        *psi.Snapshot
	node        psi.NodeRef
	score       int
	expect      bool
}

// locateKotlin finds t in the Kotlin files of an archive: the files that
// declare its top-level name in its package, trying each split of its
// name into a package and classes, the longest package first, and else
// any file that declares the name at the top level.
func locateKotlin(archive string, idx *index, t Target, read func(entry string) (string, error)) (Location, bool) {
	parts := strings.Split(t.Name, ".")
	var entries []string
	var path []string // the names from the top-level declaration's
	for i := len(parts) - 1; i >= 0; i-- {
		pkg := strings.Join(parts[:i], ".")
		if es := idx.top[topKey{pkg, parts[i]}]; len(es) > 0 {
			entries, path = es, parts[i:]
			break
		}
	}
	if entries == nil && t.Kind != "class" && t.Kind != "constructor" {
		last := parts[len(parts)-1]
		entries, path = idx.byName[last], []string{last}
	}
	if entries == nil {
		return Location{}, false
	}
	var cands []candidate
	for _, e := range entries {
		text, err := read(e)
		if err != nil {
			continue
		}
		snap := psi.Parse(e, text)
		for _, n := range members(snap.RootNode(), path, t.Kind) {
			cands = append(cands, candidate{entry: e, text: text, snap: snap, node: n, score: score(n, t), expect: isExpect(n)})
		}
	}
	if len(cands) == 0 {
		return Location{}, false
	}
	rank := func(c candidate) int {
		s := c.score * 2
		if !c.expect {
			s++ // the implementation, as an editor goes to the actual
		}
		return s
	}
	best := slices.MaxFunc(cands, func(a, b candidate) int { return cmp.Compare(rank(a), rank(b)) })
	// slices.MaxFunc returns the first of equals: the first file's.
	loc := Location{Archive: archive, Entry: best.entry, Text: best.text, Range: nameRange(best.node), Snap: best.snap, Node: best.node.ID()}
	if hasKDoc(best.node) {
		loc.Doc = best.node
	} else {
		for _, c := range cands {
			if c.score == best.score && hasKDoc(c.node) {
				loc.Doc = c.node
				break
			}
		}
	}
	return loc, true
}

// members returns the declarations along path from root: its top-level
// declarations named path[0], their members named path[1], and so on,
// the last of kind's.
func members(root psi.NodeRef, path []string, kind string) []psi.NodeRef {
	level := []psi.NodeRef{root}
	for i, name := range path {
		last := i == len(path)-1
		var next []psi.NodeRef
		for _, owner := range level {
			for _, d := range declarations(owner) {
				if d.Name() != name {
					continue
				}
				if !last {
					if isClass(d) {
						next = append(next, d)
					}
					continue
				}
				if kindMatches(d, kind) {
					next = append(next, d)
				}
			}
		}
		if len(next) == 0 && !last {
			return nil
		}
		level = next
	}
	return level
}

// declarations returns the declarations in a file or class: its
// top-level ones, or its members and a primary constructor's properties.
func declarations(owner psi.NodeRef) []psi.NodeRef {
	var out []psi.NodeRef
	body := owner
	if owner.NumericKind() != psi.KindFILE {
		for _, p := range owner.ChildKind(psi.KindPRIMARY_CONSTRUCTOR).Parameters() {
			if p.ChildKind(psi.KindVAL_KEYWORD).Valid() || p.ChildKind(psi.KindVAR_KEYWORD).Valid() {
				out = append(out, p)
			}
		}
		body = owner.ChildKind(psi.KindCLASS_BODY)
	}
	for c := body.FirstChild(); c.Valid(); c = c.NextSibling() {
		switch c.NumericKind() {
		case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindFUN, psi.KindPROPERTY, psi.KindTYPEALIAS, psi.KindENUM_ENTRY:
			out = append(out, c)
		}
	}
	// A companion's members are the class's to its users.
	for _, c := range slices.Clone(out) {
		if c.NumericKind() == psi.KindOBJECT_DECLARATION && hasModifier(c, psi.KindCOMPANION_KEYWORD) {
			out = append(out, declarations(c)...)
		}
	}
	return out
}

func isClass(d psi.NodeRef) bool {
	k := d.NumericKind()
	return k == psi.KindCLASS || k == psi.KindOBJECT_DECLARATION
}

func kindMatches(d psi.NodeRef, kind string) bool {
	switch d.NumericKind() {
	case psi.KindCLASS, psi.KindOBJECT_DECLARATION, psi.KindTYPEALIAS:
		return kind == "class" || kind == "constructor"
	case psi.KindFUN:
		return kind == "function"
	case psi.KindPROPERTY, psi.KindVALUE_PARAMETER, psi.KindENUM_ENTRY:
		return kind == "property"
	}
	return false
}

// score is how well declaration d matches t's parameters and receiver.
func score(d psi.NodeRef, t Target) int {
	s := 0
	if t.Receiver != "" && simpleType(d.ReceiverType().Text()) == t.Receiver {
		s += 4
	}
	if d.NumericKind() != psi.KindFUN || !t.Params {
		return s
	}
	ps := d.Parameters()
	if len(ps) != len(t.Parameters) {
		return s
	}
	s += 4
	for i, p := range ps {
		vararg := hasModifier(p, psi.KindVARARG_KEYWORD)
		if i < len(t.Varargs) && t.Varargs[i] == vararg {
			s++
		}
		if simpleType(p.TypeReference().Text()) == t.Parameters[i] {
			s += 2
		}
	}
	return s
}

// simpleType returns the simple name of a type as written: List for
// kotlin.collections.List<T>?, Function1 for (T) -> R.
func simpleType(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "suspend ")
	for strings.HasPrefix(text, "@") { // annotations
		i := strings.IndexByte(text, ' ')
		if i < 0 {
			return ""
		}
		text = strings.TrimSpace(text[i:])
	}
	if strings.HasPrefix(text, "(") {
		depth := 0
		for i, c := range text {
			switch c {
			case '(', '<':
				depth++
			case ')', '>':
				depth--
			}
			if depth == 0 {
				inner := strings.TrimSpace(text[1:i])
				if strings.Contains(text[i:], "->") {
					n := 0
					if inner != "" {
						n = 1 + topLevelCommas(inner)
					}
					return "Function" + itoa(n)
				}
				return simpleType(inner) // (T)?
			}
		}
		return ""
	}
	if i := strings.IndexAny(text, "<?"); i >= 0 {
		text = text[:i]
	}
	if i := strings.LastIndexByte(text, '.'); i >= 0 {
		text = text[i+1:]
	}
	return strings.Trim(strings.TrimSpace(text), "`")
}

func topLevelCommas(s string) int {
	n, depth := 0, 0
	for _, c := range s {
		switch c {
		case '(', '<':
			depth++
		case ')', '>':
			depth--
		case ',':
			if depth == 0 {
				n++
			}
		}
	}
	return n
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

func hasModifier(d psi.NodeRef, k psi.Kind) bool {
	return d.ChildKind(psi.KindMODIFIER_LIST).ChildKind(k).Valid()
}

// isExpect reports whether d is an expect declaration, or a member of
// one.
func isExpect(d psi.NodeRef) bool {
	for ; d.Valid(); d = d.Parent() {
		if hasModifier(d, psi.KindEXPECT_KEYWORD) {
			return true
		}
	}
	return false
}

func hasKDoc(d psi.NodeRef) bool {
	for c := d.FirstChild(); c.Valid(); c = c.NextSibling() {
		if c.NumericKind() == psi.KindKDOC {
			return true
		}
		if !c.IsTrivia() {
			break
		}
	}
	return false
}

// nameRange returns the range of a declaration's name, or its start.
func nameRange(d psi.NodeRef) psi.Range {
	if id := d.NameIdentifier(); id.Valid() {
		return id.Range()
	}
	if d.NumericKind() == psi.KindOBJECT_DECLARATION {
		if o := d.ChildKind(psi.KindOBJECT_KEYWORD); o.Valid() {
			return o.Range()
		}
	}
	r := d.Range()
	return psi.Range{Start: r.Start, End: r.Start}
}

package lsp

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/barteo/kotlin-compiler-go/analysis"
	"github.com/barteo/kotlin-compiler-go/internal/ide"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/jsonrpc"
	"github.com/barteo/kotlin-compiler-go/internal/lsp/protocol"
	"github.com/barteo/kotlin-compiler-go/psi"
)

// Completion (docs/lsp.md, milestone L4) checks the document as it is
// now, with a probe at the caret (ide.CompletionAt), in the module the
// file belongs to: it doesn't wait for or change the module's updates.

// completionData is a completion item's data, for completionItem/resolve:
// where its declaration starts.
type completionData struct {
	Path string `json:"path"`
	Off  int    `json:"off"`
}

// ownerModule returns the module path belongs to, or nil.
func (s *Server) ownerModule(path string) *module {
	s.mu.Lock()
	ws := slices.Clone(s.ws)
	s.mu.Unlock()
	for _, w := range ws {
		w.mu.Lock()
		var m *module
		if w.proj != nil {
			if pm := w.proj.ModuleOf(path); pm != nil {
				m = w.mods[pm.Name]
			}
		}
		w.mu.Unlock()
		if m != nil {
			return m
		}
	}
	return nil
}

// snapshots returns m's sources: the open documents' snapshots and the
// other files' on disk, with snap in place of its file's.
func (m *module) snapshots(snap *psi.Snapshot) ([]*psi.Snapshot, error) {
	paths, err := m.listSources()
	if err != nil {
		return nil, err
	}
	s := m.w.s
	open := s.openSnapshots()
	if snap != nil {
		open[snap.Path()] = snap
	}
	snaps := make([]*psi.Snapshot, 0, len(paths))
	for _, p := range paths {
		if sn := open[p]; sn != nil {
			snaps = append(snaps, sn)
		} else if sn, err := s.disk.snapshot(p); err == nil {
			snaps = append(snaps, sn)
		}
	}
	return snaps, nil
}

func (s *Server) completion(ctx context.Context, r *jsonrpc.Request) (any, error) {
	var p protocol.CompletionParams
	if err := r.Unmarshal(&p); err != nil {
		return nil, err
	}
	d, enc, err := s.doc(p.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	off, err := d.Offset(pos(p.Position), enc)
	if err != nil {
		return nil, jsonrpc.Errorf(jsonrpc.CodeInvalidParams, "%v", err)
	}
	s.mu.Lock()
	opts := ide.CompletionOptions{Snippets: s.snippets}
	s.mu.Unlock()
	cc := ide.CompletionAt(d, off)
	list := &protocol.CompletionList{Items: []protocol.CompletionItem{}}
	if cc.None {
		return list, nil
	}
	m := s.ownerModule(d.Path)
	var items []ide.CompletionItem
	switch {
	case cc.Import:
		if m == nil {
			return list, nil
		}
		items, err = m.importItems(cc)
	default:
		var c *analysis.Completion
		if m != nil {
			c, err = m.complete(ctx, cc)
		}
		if err == nil {
			items, list.IsIncomplete = ide.Complete(cc, c, opts)
		}
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		s.logMessage(protocol.MessageWarning, "completion: "+err.Error())
		return list, nil
	}
	replace := rng(d, cc.Replace, enc)
	for _, it := range items {
		list.Items = append(list.Items, completionItem(d, it, replace, enc))
	}
	return list, nil
}

// complete checks m's sources with cc's probe.
func (m *module) complete(ctx context.Context, cc ide.CompletionContext) (*analysis.Completion, error) {
	snaps, err := m.snapshots(psi.Parse(cc.Path, cc.ProbeText))
	if err != nil {
		return nil, err
	}
	return m.sess.Complete(ctx, snaps, cc.Path, cc.Prefix)
}

// importItems lists what an import or package directive may name at cc.
func (m *module) importItems(cc ide.CompletionContext) ([]ide.CompletionItem, error) {
	snaps, err := m.snapshots(nil)
	if err != nil {
		return nil, err
	}
	members, err := m.sess.PackageMembers(snaps, cc.Qualifier)
	if err != nil {
		return nil, err
	}
	return ide.CompleteImport(cc, members), nil
}

// completionKinds are the protocol's kinds of ide's.
var completionKinds = map[string]int{
	"local": protocol.CompletionVariable, "parameter": protocol.CompletionVariable, "namedArgument": protocol.CompletionVariable,
	"property": protocol.CompletionProperty, "function": protocol.CompletionFunction, "method": protocol.CompletionMethod,
	"class": protocol.CompletionClass, "interface": protocol.CompletionInterface, "object": protocol.CompletionStruct,
	"enum": protocol.CompletionEnum, "annotation": protocol.CompletionInterface, "typealias": protocol.CompletionClass,
	"typeParameter": protocol.CompletionTypeParameter, "package": protocol.CompletionModule, "keyword": protocol.CompletionKeyword,
}

func completionItem(d *ide.Document, it ide.CompletionItem, replace protocol.Range, enc ide.Encoding) protocol.CompletionItem {
	ci := protocol.CompletionItem{
		Label: it.Label, Kind: completionKinds[it.Kind], Detail: it.Detail, SortText: it.Sort, FilterText: it.Filter,
		TextEdit: &protocol.TextEdit{Range: replace, NewText: it.Insert},
	}
	if it.Snippet {
		ci.InsertTextFormat = protocol.InsertSnippet
	}
	if it.Deprecated {
		ci.Tags = []int{protocol.CompletionItemTagDeprecated}
	}
	if it.Import != nil {
		ci.AdditionalTextEdits = []protocol.TextEdit{{Range: rng(d, it.Import.Range, enc), NewText: it.Import.Text}}
	}
	if it.Decl != nil {
		ci.Data, _ = json.Marshal(completionData{it.Decl.Path, it.Decl.Off})
	}
	return ci
}

// resolveCompletion adds an item's documentation: its declaration's
// header and KDoc.
func (s *Server) resolveCompletion(_ context.Context, r *jsonrpc.Request) (any, error) {
	var ci protocol.CompletionItem
	if err := r.Unmarshal(&ci); err != nil {
		return nil, err
	}
	var data completionData
	if len(ci.Data) == 0 || json.Unmarshal(ci.Data, &data) != nil || data.Path == "" {
		return ci, nil
	}
	snap := s.openSnapshots()[data.Path]
	if snap == nil {
		var err error
		if snap, err = s.disk.snapshot(data.Path); err != nil {
			return ci, nil
		}
	}
	if doc := ide.CompletionDoc(snap, data.Off); doc != "" {
		ci.Documentation = &protocol.MarkupContent{Kind: protocol.MarkupKindMarkdown, Value: strings.TrimSpace(doc)}
	}
	return ci, nil
}

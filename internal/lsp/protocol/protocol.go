// Package protocol holds the Language Server Protocol 3.17 types that
// kc-lsp uses, written by hand as features need them. Field names and
// JSON names follow the specification.
package protocol

import "encoding/json"

// DocumentURI is a file URI.
type DocumentURI string

// Position encodings (PositionEncodingKind).
const (
	EncodingUTF8  = "utf-8"
	EncodingUTF16 = "utf-16"
)

// Position is a zero-based line and character offset; characters count in
// the negotiated position encoding.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a half-open range of positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a range in a document.
type Location struct {
	URI   DocumentURI `json:"uri"`
	Range Range       `json:"range"`
}

// ---- lifecycle ----

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type InitializeParams struct {
	ProcessID             *int               `json:"processId"`
	ClientInfo            *ClientInfo        `json:"clientInfo,omitempty"`
	RootURI               DocumentURI        `json:"rootUri,omitempty"`
	Capabilities          ClientCapabilities `json:"capabilities"`
	InitializationOptions json.RawMessage    `json:"initializationOptions,omitempty"`
	WorkspaceFolders      []WorkspaceFolder  `json:"workspaceFolders,omitempty"`
}

type WorkspaceFolder struct {
	URI  DocumentURI `json:"uri"`
	Name string      `json:"name"`
}

type ClientCapabilities struct {
	General      *GeneralClientCapabilities      `json:"general,omitempty"`
	TextDocument *TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Workspace    *WorkspaceClientCapabilities    `json:"workspace,omitempty"`
}

type WorkspaceClientCapabilities struct {
	DidChangeWatchedFiles *DynamicRegistrationCapabilities `json:"didChangeWatchedFiles,omitempty"`
}

type DynamicRegistrationCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type GeneralClientCapabilities struct {
	PositionEncodings []string `json:"positionEncodings,omitempty"`
}

type TextDocumentClientCapabilities struct {
	DocumentSymbol *DocumentSymbolClientCapabilities `json:"documentSymbol,omitempty"`
}

type DocumentSymbolClientCapabilities struct {
	HierarchicalDocumentSymbolSupport bool `json:"hierarchicalDocumentSymbolSupport,omitempty"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   *ServerInfo        `json:"serverInfo,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// Text document sync kinds.
const (
	SyncNone        = 0
	SyncFull        = 1
	SyncIncremental = 2
)

type TextDocumentSyncOptions struct {
	OpenClose bool `json:"openClose"`
	Change    int  `json:"change"`
	Save      bool `json:"save,omitempty"`
}

type ServerCapabilities struct {
	PositionEncoding       string                   `json:"positionEncoding,omitempty"`
	TextDocumentSync       *TextDocumentSyncOptions `json:"textDocumentSync,omitempty"`
	DocumentSymbolProvider bool                     `json:"documentSymbolProvider,omitempty"`
	FoldingRangeProvider   bool                     `json:"foldingRangeProvider,omitempty"`
	SelectionRangeProvider bool                     `json:"selectionRangeProvider,omitempty"`
	SemanticTokensProvider *SemanticTokensOptions   `json:"semanticTokensProvider,omitempty"`

	DefinitionProvider        bool `json:"definitionProvider,omitempty"`
	TypeDefinitionProvider    bool `json:"typeDefinitionProvider,omitempty"`
	ReferencesProvider        bool `json:"referencesProvider,omitempty"`
	DocumentHighlightProvider bool `json:"documentHighlightProvider,omitempty"`
}

// ---- document synchronization ----

type TextDocumentItem struct {
	URI        DocumentURI `json:"uri"`
	LanguageID string      `json:"languageId"`
	Version    int         `json:"version"`
	Text       string      `json:"text"`
}

type TextDocumentIdentifier struct {
	URI DocumentURI `json:"uri"`
}

type VersionedTextDocumentIdentifier struct {
	URI     DocumentURI `json:"uri"`
	Version int         `json:"version"`
}

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// TextDocumentContentChangeEvent replaces Range with Text, or the whole
// document when Range is nil.
type TextDocumentContentChangeEvent struct {
	Range *Range `json:"range,omitempty"`
	Text  string `json:"text"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// ---- workspace ----

// File change types.
const (
	FileCreated = 1
	FileChanged = 2
	FileDeleted = 3
)

type FileEvent struct {
	URI  DocumentURI `json:"uri"`
	Type int         `json:"type"`
}

type DidChangeWatchedFilesParams struct {
	Changes []FileEvent `json:"changes"`
}

type FileSystemWatcher struct {
	GlobPattern string `json:"globPattern"`
}

type DidChangeWatchedFilesRegistrationOptions struct {
	Watchers []FileSystemWatcher `json:"watchers"`
}

type Registration struct {
	ID              string `json:"id"`
	Method          string `json:"method"`
	RegisterOptions any    `json:"registerOptions,omitempty"`
}

type RegistrationParams struct {
	Registrations []Registration `json:"registrations"`
}

// Message types.
const (
	MessageError   = 1
	MessageWarning = 2
	MessageInfo    = 3
	MessageLog     = 4
)

type LogMessageParams struct {
	Type    int    `json:"type"`
	Message string `json:"message"`
}

// ---- diagnostics ----

// Diagnostic severities.
const (
	SeverityError       = 1
	SeverityWarning     = 2
	SeverityInformation = 3
	SeverityHint        = 4
)

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

type PublishDiagnosticsParams struct {
	URI         DocumentURI  `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// ---- document symbols ----

type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// SymbolKind values.
const (
	SymbolFile          = 1
	SymbolPackage       = 4
	SymbolClass         = 5
	SymbolMethod        = 6
	SymbolProperty      = 7
	SymbolConstructor   = 9
	SymbolEnum          = 10
	SymbolInterface     = 11
	SymbolFunction      = 12
	SymbolVariable      = 13
	SymbolConstant      = 14
	SymbolEnumMember    = 22
	SymbolObject        = 19
	SymbolTypeParameter = 26
)

type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SymbolInformation is the flat form, for clients without hierarchical
// document symbols.
type SymbolInformation struct {
	Name          string   `json:"name"`
	Kind          int      `json:"kind"`
	Location      Location `json:"location"`
	ContainerName string   `json:"containerName,omitempty"`
}

// ---- folding ----

type FoldingRangeParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// Folding range kinds.
const (
	FoldingComment = "comment"
	FoldingImports = "imports"
	FoldingRegion  = "region"
)

type FoldingRange struct {
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Kind      string `json:"kind,omitempty"`
}

// ---- selection ranges ----

type SelectionRangeParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Positions    []Position             `json:"positions"`
}

type SelectionRange struct {
	Range  Range           `json:"range"`
	Parent *SelectionRange `json:"parent,omitempty"`
}

// ---- semantic tokens ----

type SemanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

type SemanticTokensOptions struct {
	Legend SemanticTokensLegend `json:"legend"`
	Full   bool                 `json:"full"`
}

type SemanticTokensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type SemanticTokens struct {
	Data []uint32 `json:"data"`
}

// ---- navigation ----

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

// Document highlight kinds (DocumentHighlightKind).
const (
	HighlightText  = 1
	HighlightRead  = 2
	HighlightWrite = 3
)

type DocumentHighlight struct {
	Range Range `json:"range"`
	Kind  int   `json:"kind,omitempty"`
}

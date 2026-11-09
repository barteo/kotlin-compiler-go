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
	Completion     *CompletionClientCapabilities     `json:"completion,omitempty"`
}

type CompletionClientCapabilities struct {
	CompletionItem *CompletionItemClientCapabilities `json:"completionItem,omitempty"`
}

type CompletionItemClientCapabilities struct {
	SnippetSupport bool `json:"snippetSupport,omitempty"`
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
	HoverProvider             bool `json:"hoverProvider,omitempty"`
	ImplementationProvider    bool `json:"implementationProvider,omitempty"`
	WorkspaceSymbolProvider   bool `json:"workspaceSymbolProvider,omitempty"`
	InlayHintProvider         bool `json:"inlayHintProvider,omitempty"`

	CompletionProvider    *CompletionOptions    `json:"completionProvider,omitempty"`
	SignatureHelpProvider *SignatureHelpOptions `json:"signatureHelpProvider,omitempty"`
	RenameProvider        *RenameOptions        `json:"renameProvider,omitempty"`
	CallHierarchyProvider bool                  `json:"callHierarchyProvider,omitempty"`
	CodeActionProvider    *CodeActionOptions    `json:"codeActionProvider,omitempty"`

	DocumentFormattingProvider       bool                             `json:"documentFormattingProvider,omitempty"`
	DocumentRangeFormattingProvider  bool                             `json:"documentRangeFormattingProvider,omitempty"`
	DocumentOnTypeFormattingProvider *DocumentOnTypeFormattingOptions `json:"documentOnTypeFormattingProvider,omitempty"`
}

type DocumentOnTypeFormattingOptions struct {
	FirstTriggerCharacter string   `json:"firstTriggerCharacter"`
	MoreTriggerCharacter  []string `json:"moreTriggerCharacter,omitempty"`
}

type FormattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}

type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      FormattingOptions      `json:"options"`
}

type DocumentRangeFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Options      FormattingOptions      `json:"options"`
}

type DocumentOnTypeFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Ch           string                 `json:"ch"`
	Options      FormattingOptions      `json:"options"`
}

type CallHierarchyItem struct {
	Name           string      `json:"name"`
	Kind           int         `json:"kind"`
	Detail         string      `json:"detail,omitempty"`
	URI            DocumentURI `json:"uri"`
	Range          Range       `json:"range"`
	SelectionRange Range       `json:"selectionRange"`
}

type CallHierarchyCallsParams struct {
	Item CallHierarchyItem `json:"item"`
}

type CallHierarchyIncomingCall struct {
	From       CallHierarchyItem `json:"from"`
	FromRanges []Range           `json:"fromRanges"`
}

type CallHierarchyOutgoingCall struct {
	To         CallHierarchyItem `json:"to"`
	FromRanges []Range           `json:"fromRanges"`
}

type RenameOptions struct {
	PrepareProvider bool `json:"prepareProvider,omitempty"`
}

type RenameParams struct {
	TextDocumentPositionParams
	NewName string `json:"newName"`
}

// PrepareRenameResult is the range and placeholder variant of
// textDocument/prepareRename's result.
type PrepareRenameResult struct {
	Range       Range  `json:"range"`
	Placeholder string `json:"placeholder"`
}

type WorkspaceEdit struct {
	Changes map[DocumentURI][]TextEdit `json:"changes"`
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
	Range    Range    `json:"range"`
	Severity int      `json:"severity,omitempty"`
	Code     string   `json:"code,omitempty"`
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
	Tags     []int    `json:"tags,omitempty"`
	Data     []string `json:"data,omitempty"`
}

// DiagnosticTagUnnecessary marks unused or unnecessary code.
const DiagnosticTagUnnecessary = 1

// ---- code actions ----

// Code action kinds.
const (
	CodeActionQuickFix        = "quickfix"
	CodeActionOrganizeImports = "source.organizeImports"
)

type CodeActionOptions struct {
	CodeActionKinds []string `json:"codeActionKinds,omitempty"`
}

type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

// CodeActionContext's Diagnostics are the client's; kc offers fixes of
// its own current diagnostics in the range instead.
type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
	Only        []string     `json:"only,omitempty"`
}

type CodeAction struct {
	Title       string         `json:"title"`
	Kind        string         `json:"kind,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	IsPreferred bool           `json:"isPreferred,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
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

type WorkspaceSymbolParams struct {
	Query string `json:"query"`
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

// Inlay hint kinds (InlayHintKind).
const (
	InlayHintType      = 1
	InlayHintParameter = 2
)

type InlayHintParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
}

type InlayHint struct {
	Position     Position `json:"position"`
	Label        string   `json:"label"`
	Kind         int      `json:"kind,omitempty"`
	PaddingLeft  bool     `json:"paddingLeft,omitempty"`
	PaddingRight bool     `json:"paddingRight,omitempty"`
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

// MarkupKindMarkdown is the kind of Markdown MarkupContent.
const MarkupKindMarkdown = "markdown"

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// ---- completion ----

type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
	ResolveProvider   bool     `json:"resolveProvider,omitempty"`
}

type CompletionContext struct {
	TriggerKind      int    `json:"triggerKind"`
	TriggerCharacter string `json:"triggerCharacter,omitempty"`
}

type CompletionParams struct {
	TextDocumentPositionParams
	Context *CompletionContext `json:"context,omitempty"`
}

// Completion item kinds (CompletionItemKind).
const (
	CompletionMethod        = 2
	CompletionFunction      = 3
	CompletionConstructor   = 4
	CompletionField         = 5
	CompletionVariable      = 6
	CompletionClass         = 7
	CompletionInterface     = 8
	CompletionModule        = 9
	CompletionProperty      = 10
	CompletionEnum          = 13
	CompletionKeyword       = 14
	CompletionStruct        = 22
	CompletionTypeParameter = 25
)

// Insert text formats (InsertTextFormat).
const (
	InsertPlainText = 1
	InsertSnippet   = 2
)

// CompletionItemTagDeprecated marks a deprecated item.
const CompletionItemTagDeprecated = 1

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

type CompletionItem struct {
	Label               string          `json:"label"`
	Kind                int             `json:"kind,omitempty"`
	Tags                []int           `json:"tags,omitempty"`
	Detail              string          `json:"detail,omitempty"`
	Documentation       *MarkupContent  `json:"documentation,omitempty"`
	SortText            string          `json:"sortText,omitempty"`
	FilterText          string          `json:"filterText,omitempty"`
	InsertTextFormat    int             `json:"insertTextFormat,omitempty"`
	TextEdit            *TextEdit       `json:"textEdit,omitempty"`
	AdditionalTextEdits []TextEdit      `json:"additionalTextEdits,omitempty"`
	Data                json.RawMessage `json:"data,omitempty"`
}

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// ---- signature help ----

type SignatureHelpOptions struct {
	TriggerCharacters   []string `json:"triggerCharacters,omitempty"`
	RetriggerCharacters []string `json:"retriggerCharacters,omitempty"`
}

type ParameterInformation struct {
	// Label is the parameter's [start, end) in its signature's label,
	// in UTF-16 units.
	Label [2]int `json:"label"`
}

type SignatureInformation struct {
	Label           string                 `json:"label"`
	Documentation   *MarkupContent         `json:"documentation,omitempty"`
	Parameters      []ParameterInformation `json:"parameters"`
	ActiveParameter *int                   `json:"activeParameter,omitempty"`
}

type SignatureHelp struct {
	Signatures      []SignatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

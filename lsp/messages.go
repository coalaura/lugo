package lsp

import "encoding/json"

type Request struct {
	RPC    string          `json:"jsonrpc"`
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	RPC    string `json:"jsonrpc"`
	ID     int    `json:"id"`
	Result any    `json:"result"`
	Error  any    `json:"error,omitempty"`
}

type Notification struct {
	RPC    string          `json:"jsonrpc"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type InitializeParams struct {
	RootURI               string                `json:"rootUri"`
	InitializationOptions InitializationOptions `json:"initializationOptions"`
}

type InitializationOptions struct {
	LibraryPaths []string `json:"libraryPaths,omitempty"`
	KnownGlobals []string `json:"knownGlobals,omitempty"`
	IgnoreGlobs  []string `json:"ignoreGlobs,omitempty"`

	DiagnosticsUndefinedGlobals bool `json:"diagnosticsUndefinedGlobals"`
	DiagnosticsImplicitGlobals  bool `json:"diagnosticsImplicitGlobals"`
	DiagnosticsUnusedLocal      bool `json:"diagnosticsUnusedLocal"`
	DiagnosticsUnusedFunction   bool `json:"diagnosticsUnusedFunction"`
	DiagnosticsUnusedParameter  bool `json:"diagnosticsUnusedParameter"`
	DiagnosticsUnusedLoopVar    bool `json:"diagnosticsUnusedLoopVar"`
	DiagnosticsShadowing        bool `json:"diagnosticsShadowing"`
	DiagnosticsUnreachableCode  bool `json:"diagnosticsUnreachableCode"`
	DiagnosticsAmbiguousReturns bool `json:"diagnosticsAmbiguousReturns"`
	DiagnosticsDeprecated       bool `json:"diagnosticsDeprecated"`
	InlayHintsParameterNames    bool `json:"inlayHintsParameterNames"`
	FeaturesDocumentHighlight   bool `json:"featuresDocumentHighlight"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

type ServerCapabilities struct {
	TextDocumentSync           int                    `json:"textDocumentSync"`
	DefinitionProvider         bool                   `json:"definitionProvider"`
	HoverProvider              bool                   `json:"hoverProvider"`
	RenameProvider             any                    `json:"renameProvider"`
	ReferencesProvider         bool                   `json:"referencesProvider"`
	DocumentSymbolProvider     bool                   `json:"documentSymbolProvider"`
	WorkspaceSymbolProvider    bool                   `json:"workspaceSymbolProvider"`
	InlayHintProvider          bool                   `json:"inlayHintProvider"`
	CodeActionProvider         bool                   `json:"codeActionProvider"`
	FoldingRangeProvider       bool                   `json:"foldingRangeProvider"`
	LinkedEditingRangeProvider bool                   `json:"linkedEditingRangeProvider"`
	CallHierarchyProvider      bool                   `json:"callHierarchyProvider"`
	DocumentHighlightProvider  bool                   `json:"documentHighlightProvider,omitempty"`
	CodeLensProvider           *CodeLensOptions       `json:"codeLensProvider,omitempty"`
	SignatureHelpProvider      *SignatureHelpOptions  `json:"signatureHelpProvider,omitempty"`
	CompletionProvider         *CompletionOptions     `json:"completionProvider,omitempty"`
	SemanticTokensProvider     *SemanticTokensOptions `json:"semanticTokensProvider,omitempty"`
}

type TextDocumentItem struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
	Text    string `json:"text"`
}

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type TextDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type Position struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type OutgoingNotification struct {
	RPC    string `json:"jsonrpc"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type DiagnosticSeverity int

const (
	SeverityError       DiagnosticSeverity = 1
	SeverityWarning     DiagnosticSeverity = 2
	SeverityInformation DiagnosticSeverity = 3
	SeverityHint        DiagnosticSeverity = 4
)

type DiagnosticTag int

const (
	Unnecessary DiagnosticTag = 1
	Deprecated  DiagnosticTag = 2
)

type Diagnostic struct {
	Range              Range                          `json:"range"`
	Severity           DiagnosticSeverity             `json:"severity,omitempty"`
	Code               string                         `json:"code,omitempty"`
	Message            string                         `json:"message"`
	Tags               []DiagnosticTag                `json:"tags,omitempty"`
	RelatedInformation []DiagnosticRelatedInformation `json:"relatedInformation,omitempty"`
}

type DiagnosticRelatedInformation struct {
	Location Location `json:"location"`
	Message  string   `json:"message"`
}

type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type CompletionItemKind int

const (
	FunctionCompletion CompletionItemKind = 3
	FieldCompletion    CompletionItemKind = 5
	VariableCompletion CompletionItemKind = 6
	KeywordCompletion  CompletionItemKind = 14
)

type CompletionItemTag int

const (
	CompletionItemTagDeprecated CompletionItemTag = 1
)

type CompletionItem struct {
	Label         string              `json:"label"`
	Kind          CompletionItemKind  `json:"kind"`
	Detail        string              `json:"detail,omitempty"`
	Documentation *MarkupContent      `json:"documentation,omitempty"`
	Tags          []CompletionItemTag `json:"tags,omitempty"`
}

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type RenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	NewName      string                 `json:"newName"`
}

type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

type ReferenceParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      ReferenceContext       `json:"context"`
}

type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

type SymbolInformation struct {
	Name          string     `json:"name"`
	Kind          SymbolKind `json:"kind"`
	Location      Location   `json:"location"`
	ContainerName string     `json:"containerName,omitempty"`
}

type SymbolKind int

const (
	SymbolKindFile     SymbolKind = 1
	SymbolKindClass    SymbolKind = 5 // class for tables
	SymbolKindMethod   SymbolKind = 6
	SymbolKindField    SymbolKind = 8
	SymbolKindFunction SymbolKind = 12
	SymbolKindVariable SymbolKind = 13
)

type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           SymbolKind       `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

type ReadStdParams struct {
	URI string `json:"uri"`
}

type ReadStdResult struct {
	Content string `json:"content"`
}

type SignatureHelpOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type SignatureHelpParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type SignatureHelp struct {
	Signatures      []SignatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

type SignatureInformation struct {
	Label         string                 `json:"label"`
	Documentation *MarkupContent         `json:"documentation,omitempty"`
	Parameters    []ParameterInformation `json:"parameters,omitempty"`
}

type ParameterInformation struct {
	Label         string         `json:"label"`
	Documentation *MarkupContent `json:"documentation,omitempty"`
}

type InlayHintParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
}

type InlayHintKind int

const (
	TypeHint      InlayHintKind = 1
	ParameterHint InlayHintKind = 2
)

type InlayHint struct {
	Position     Position      `json:"position"`
	Label        string        `json:"label"`
	Kind         InlayHintKind `json:"kind,omitempty"`
	PaddingLeft  bool          `json:"paddingLeft,omitempty"`
	PaddingRight bool          `json:"paddingRight,omitempty"`
}

type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      CodeActionContext      `json:"context"`
}

type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type CodeAction struct {
	Title       string         `json:"title"`
	Kind        string         `json:"kind,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
	IsPreferred bool           `json:"isPreferred,omitempty"`
}

type CodeLensOptions struct {
	ResolveProvider bool `json:"resolveProvider,omitempty"`
}

type CodeLensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type CodeLens struct {
	Range   Range    `json:"range"`
	Command *Command `json:"command,omitempty"`
	Data    any      `json:"data,omitempty"`
}

type Command struct {
	Title     string `json:"title"`
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
}

type SemanticTokensOptions struct {
	Legend SemanticTokensLegend `json:"legend"`
	Full   bool                 `json:"full"`
}

type SemanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

type SymbolTag int

const (
	SymbolTagDeprecated SymbolTag = 1
)

type CallHierarchyPrepareParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type CallHierarchyItem struct {
	Name           string      `json:"name"`
	Kind           SymbolKind  `json:"kind"`
	Tags           []SymbolTag `json:"tags,omitempty"`
	Detail         string      `json:"detail,omitempty"`
	URI            string      `json:"uri"`
	Range          Range       `json:"range"`
	SelectionRange Range       `json:"selectionRange"`
	Data           any         `json:"data,omitempty"`
}

type CallHierarchyIncomingCallsParams struct {
	Item CallHierarchyItem `json:"item"`
}

type CallHierarchyIncomingCall struct {
	From       CallHierarchyItem `json:"from"`
	FromRanges []Range           `json:"fromRanges"`
}

type CallHierarchyOutgoingCallsParams struct {
	Item CallHierarchyItem `json:"item"`
}

type CallHierarchyOutgoingCall struct {
	To         CallHierarchyItem `json:"to"`
	FromRanges []Range           `json:"fromRanges"`
}

type SemanticTokensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type SemanticTokens struct {
	Data []uint32 `json:"data"`
}

type FoldingRangeParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type FoldingRange struct {
	StartLine      uint32 `json:"startLine"`
	StartCharacter uint32 `json:"startCharacter,omitempty"`
	EndLine        uint32 `json:"endLine"`
	EndCharacter   uint32 `json:"endCharacter,omitempty"`
	Kind           string `json:"kind,omitempty"`
}

type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type PrepareRenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type PrepareRenameResult struct {
	Range       Range  `json:"range"`
	Placeholder string `json:"placeholder"`
}

type LinkedEditingRangeParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type LinkedEditingRanges struct {
	Ranges      []Range `json:"ranges"`
	WordPattern string  `json:"wordPattern,omitempty"`
}

type DocumentHighlightParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type DocumentHighlightKind int

const (
	TextHighlight  DocumentHighlightKind = 1
	ReadHighlight  DocumentHighlightKind = 2
	WriteHighlight DocumentHighlightKind = 3
)

type DocumentHighlight struct {
	Range Range                 `json:"range"`
	Kind  DocumentHighlightKind `json:"kind,omitempty"`
}

package lsp

import "encoding/json"

type Request struct {
	RPC    string          `json:"jsonrpc"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
	ID     int             `json:"id"`
}

type Response struct {
	RPC    string `json:"jsonrpc"`
	Result any    `json:"result"`
	Error  any    `json:"error,omitempty"`
	ID     int    `json:"id"`
}

type Notification struct {
	RPC    string          `json:"jsonrpc"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type OutgoingRequest struct {
	RPC    string `json:"jsonrpc"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
	ID     int    `json:"id"`
}

type OutgoingNotification struct {
	RPC    string `json:"jsonrpc"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type ResponseError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
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

type TextEdit struct {
	NewText string `json:"newText"`
	Range   Range  `json:"range"`
}

type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes"`
}

type Command struct {
	Title     string `json:"title"`
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type WorkspaceFolder struct {
	URI  string `json:"uri"`
	Name string `json:"name"`
}

type InitializeParams struct {
	RootURI               string                `json:"rootUri"`
	WorkspaceFolders      []WorkspaceFolder     `json:"workspaceFolders,omitempty"`
	InitializationOptions InitializationOptions `json:"initializationOptions"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

type ExecuteCommandOptions struct {
	Commands []string `json:"commands"`
}

type InitializationOptions struct {
	LibraryPaths []string `json:"libraryPaths,omitempty"`
	IgnoreGlobs  []string `json:"ignoreGlobs,omitempty"`
	KnownGlobals []string `json:"knownGlobals,omitempty"`

	ParserMaxErrors int `json:"parserMaxErrors"`

	DiagUndefinedGlobals     bool `json:"diagUndefinedGlobals"`
	DiagImplicitGlobals      bool `json:"diagImplicitGlobals"`
	DiagUnusedLocal          bool `json:"diagUnusedLocal"`
	DiagUnusedFunction       bool `json:"diagUnusedFunction"`
	DiagUnusedParameter      bool `json:"diagUnusedParameter"`
	DiagUnusedLoopVar        bool `json:"diagUnusedLoopVar"`
	DiagShadowing            bool `json:"diagShadowing"`
	DiagUnreachableCode      bool `json:"diagUnreachableCode"`
	DiagAmbiguousReturns     bool `json:"diagAmbiguousReturns"`
	DiagDeprecated           bool `json:"diagDeprecated"`
	DiagDuplicateField       bool `json:"diagDuplicateField"`
	DiagUnbalancedAssignment bool `json:"diagUnbalancedAssignment"`
	DiagDuplicateLocal       bool `json:"diagDuplicateLocal"`
	DiagSelfAssignment       bool `json:"diagSelfAssignment"`
	DiagEmptyBlock           bool `json:"diagEmptyBlock"`
	DiagFormatString         bool `json:"diagFormatString"`
	DiagTypeCheck            bool `json:"diagTypeCheck"`
	DiagRedundantParameter   bool `json:"diagRedundantParameter"`
	DiagRedundantValue       bool `json:"diagRedundantValue"`
	DiagRedundantReturn      bool `json:"diagRedundantReturn"`
	DiagLoopVarMutation      bool `json:"diagLoopVarMutation"`
	DiagIncorrectVararg      bool `json:"diagIncorrectVararg"`
	DiagShadowingLoopVar     bool `json:"diagShadowingLoopVar"`
	DiagUnreachableElse      bool `json:"diagUnreachableElse"`
	DiagUsedIgnoredVar       bool `json:"diagUsedIgnoredVar"`

	InlayParamHints    bool `json:"inlayParamHints"`
	InlaySuppressMatch bool `json:"inlaySuppressMatch"`
	InlayImplicitSelf  bool `json:"inlayImplicitSelf"`

	FeatureDocHighlight   bool `json:"featureDocHighlight"`
	FeatureHoverEval      bool `json:"featureHoverEval"`
	FeatureCodeLens       bool `json:"featureCodeLens"`
	FeatureFormatting     bool `json:"featureFormatting"`
	FormatOpinionated     bool `json:"formatOpinionated"`
	SuggestFunctionParams bool `json:"suggestFunctionParams"`
	FeatureFormatAlerts   bool `json:"featureFormatAlerts"`

	FeatureFiveM             bool `json:"featureFiveM"`
	DiagFiveMUnaccountedFile bool `json:"diagFiveMUnaccountedFile"`
	DiagFiveMUnknownExport   bool `json:"diagFiveMUnknownExport"`
	DiagFiveMUnknownResource bool `json:"diagFiveMUnknownResource"`
}

type ServerCapabilities struct {
	CodeLensProvider                *CodeLensOptions       `json:"codeLensProvider,omitempty"`
	SignatureHelpProvider           *SignatureHelpOptions  `json:"signatureHelpProvider,omitempty"`
	CompletionProvider              *CompletionOptions     `json:"completionProvider,omitempty"`
	SemanticTokensProvider          *SemanticTokensOptions `json:"semanticTokensProvider,omitempty"`
	ExecuteCommandProvider          *ExecuteCommandOptions `json:"executeCommandProvider,omitempty"`
	RenameProvider                  any                    `json:"renameProvider"`
	CodeActionProvider              any                    `json:"codeActionProvider"`
	TextDocumentSync                int                    `json:"textDocumentSync"`
	DefinitionProvider              bool                   `json:"definitionProvider"`
	HoverProvider                   bool                   `json:"hoverProvider"`
	ReferencesProvider              bool                   `json:"referencesProvider"`
	DocumentSymbolProvider          bool                   `json:"documentSymbolProvider"`
	WorkspaceSymbolProvider         bool                   `json:"workspaceSymbolProvider"`
	InlayHintProvider               bool                   `json:"inlayHintProvider"`
	FoldingRangeProvider            bool                   `json:"foldingRangeProvider"`
	SelectionRangeProvider          bool                   `json:"selectionRangeProvider,omitempty"`
	LinkedEditingRangeProvider      bool                   `json:"linkedEditingRangeProvider"`
	CallHierarchyProvider           bool                   `json:"callHierarchyProvider"`
	DocumentHighlightProvider       bool                   `json:"documentHighlightProvider,omitempty"`
	DocumentFormattingProvider      bool                   `json:"documentFormattingProvider,omitempty"`
	DocumentRangeFormattingProvider bool                   `json:"documentRangeFormattingProvider,omitempty"`
}

type TextDocumentItem struct {
	URI     string `json:"uri"`
	Text    string `json:"text"`
	Version int    `json:"version"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DidChangeConfigurationParams struct {
	Settings InitializationOptions `json:"settings"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type TextDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

type DidChangeWatchedFilesParams struct {
	Changes []FileEvent `json:"changes"`
}

type FileEvent struct {
	URI  string `json:"uri"`
	Type int    `json:"type"`
}

type ExecuteCommandParams struct {
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
}

type ApplyWorkspaceEditParams struct {
	Label string        `json:"label,omitempty"`
	Edit  WorkspaceEdit `json:"edit"`
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
	Message            string                         `json:"message"`
	Code               string                         `json:"code,omitempty"`
	Tags               []DiagnosticTag                `json:"tags,omitempty"`
	RelatedInformation []DiagnosticRelatedInformation `json:"relatedInformation,omitempty"`
	Data               any                            `json:"data,omitempty"`
	Range              Range                          `json:"range"`
	Severity           DiagnosticSeverity             `json:"severity,omitempty"`
}

type DiagnosticRelatedInformation struct {
	Message  string   `json:"message"`
	Location Location `json:"location"`
}

type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
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

type CompletionList struct {
	Items        []CompletionItem `json:"items"`
	IsIncomplete bool             `json:"isIncomplete"`
}

type InsertTextFormat int

const (
	PlainTextTextFormat InsertTextFormat = 1
	SnippetTextFormat   InsertTextFormat = 2
)

type CompletionItem struct {
	Label            string              `json:"label"`
	Detail           string              `json:"detail,omitempty"`
	SortText         string              `json:"sortText,omitempty"`
	Documentation    *MarkupContent      `json:"documentation,omitempty"`
	Tags             []CompletionItemTag `json:"tags,omitempty"`
	Kind             CompletionItemKind  `json:"kind"`
	InsertText       string              `json:"insertText,omitempty"`
	InsertTextFormat InsertTextFormat    `json:"insertTextFormat,omitempty"`
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
	Label        string        `json:"label"`
	Tooltip      string        `json:"tooltip,omitempty"`
	Position     Position      `json:"position"`
	Kind         InlayHintKind `json:"kind,omitempty"`
	PaddingLeft  bool          `json:"paddingLeft,omitempty"`
	PaddingRight bool          `json:"paddingRight,omitempty"`
}

type CodeActionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Context      CodeActionContext      `json:"context"`
	Range        Range                  `json:"range"`
}

type CodeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type CodeAction struct {
	Title       string         `json:"title"`
	Kind        string         `json:"kind,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
	Edit        *WorkspaceEdit `json:"edit,omitempty"`
	Data        any            `json:"data,omitempty"`
	IsPreferred bool           `json:"isPreferred,omitempty"`
}

type CodeLensOptions struct {
	ResolveProvider bool `json:"resolveProvider,omitempty"`
}

type CodeLensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type CodeLens struct {
	Command *Command `json:"command,omitempty"`
	Data    any      `json:"data,omitempty"`
	Range   Range    `json:"range"`
}

type RenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	NewName      string                 `json:"newName"`
	Position     Position               `json:"position"`
}

type PrepareRenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type PrepareRenameResult struct {
	Placeholder string `json:"placeholder"`
	Range       Range  `json:"range"`
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

type SymbolTag int

const (
	SymbolTagDeprecated SymbolTag = 1
)

type SymbolInformation struct {
	Name          string     `json:"name"`
	ContainerName string     `json:"containerName,omitempty"`
	Location      Location   `json:"location"`
	Kind          SymbolKind `json:"kind"`
}

type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Children       []DocumentSymbol `json:"children,omitempty"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Kind           SymbolKind       `json:"kind"`
}

type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

type ReferenceParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Context      ReferenceContext       `json:"context"`
	Position     Position               `json:"position"`
}

type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
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

type SemanticTokensOptions struct {
	Legend SemanticTokensLegend `json:"legend"`
	Full   bool                 `json:"full"`
}

type SemanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

type SemanticTokensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type SemanticTokens struct {
	Data []uint32 `json:"data"`
}

type CallHierarchyPrepareParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type CallHierarchyItem struct {
	Name           string      `json:"name"`
	Detail         string      `json:"detail,omitempty"`
	URI            string      `json:"uri"`
	Data           any         `json:"data,omitempty"`
	Tags           []SymbolTag `json:"tags,omitempty"`
	Range          Range       `json:"range"`
	SelectionRange Range       `json:"selectionRange"`
	Kind           SymbolKind  `json:"kind"`
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

type DocumentFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Options      FormattingOptions      `json:"options"`
}

type DocumentRangeFormattingParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Options      FormattingOptions      `json:"options"`
}

type FormattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}

type FoldingRangeParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type FoldingRange struct {
	Kind           string `json:"kind,omitempty"`
	StartLine      uint32 `json:"startLine"`
	StartCharacter uint32 `json:"startCharacter,omitempty"`
	EndLine        uint32 `json:"endLine"`
	EndCharacter   uint32 `json:"endCharacter,omitempty"`
}

type SelectionRangeParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Positions    []Position             `json:"positions"`
}

type SelectionRange struct {
	Range  Range           `json:"range"`
	Parent *SelectionRange `json:"parent,omitempty"`
}

type LinkedEditingRangeParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type LinkedEditingRanges struct {
	WordPattern string  `json:"wordPattern,omitempty"`
	Ranges      []Range `json:"ranges"`
}

type ReadStdParams struct {
	URI string `json:"uri"`
}

type ReadStdResult struct {
	Content string `json:"content"`
}

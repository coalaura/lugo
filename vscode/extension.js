const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const vscode = require("vscode");
const { LanguageClient } = require("vscode-languageclient/node");

let client, restarting, indexing, debounce;

async function restartClient(context) {
	if (restarting) {
		return;
	}

	restarting = true;

	try {
		if (client) {
			await client.stop();
		}

		await startClient(context);
	} catch {}

	restarting = false;
}

function buildInitializationOptions() {
	let ignoreGlobs = [],
		diagIgnoreGlobs = [],
		libraryPaths = [],
		knownGlobals = [],
		bannedSymbols = {};

	const folders = vscode.workspace.workspaceFolders || [],
		scopes = folders.map(folder => folder.uri);

	if (scopes.length === 0) {
		scopes.push(null);
	}

	const activeEditor = vscode.window.activeTextEditor;

	if (activeEditor?.document) {
		const activeUri = activeEditor.document.uri;

		if (!scopes.some(scope => scope && scope.toString() === activeUri.toString())) {
			scopes.push(activeUri);
		}
	}

	const primaryScope = activeEditor?.document?.uri || scopes[0],
		primaryLugoConfig = vscode.workspace.getConfiguration("lugo", primaryScope);

	for (const scope of scopes) {
		const filesConfig = vscode.workspace.getConfiguration("files", scope),
			searchConfig = vscode.workspace.getConfiguration("search", scope),
			lugoConfig = vscode.workspace.getConfiguration("lugo", scope);

		const folderIgnoreGlobs = lugoConfig.get("workspace.ignoreGlobs") || [],
			folderDiagIgnoreGlobs = lugoConfig.get("diagnostics.ignoreGlobs") || [],
			nativeExcludes = {
				...(filesConfig.get("exclude") || {}),
				...(searchConfig.get("exclude") || {}),
			};

		for (const [key, val] of Object.entries(nativeExcludes)) {
			if (val === true) {
				folderIgnoreGlobs.push(key);
			}
		}

		ignoreGlobs.push(...folderIgnoreGlobs);
		diagIgnoreGlobs.push(...folderDiagIgnoreGlobs);
		libraryPaths.push(...(lugoConfig.get("workspace.libraryPaths") || []));
		knownGlobals.push(...(lugoConfig.get("environment.knownGlobals") || []));

		Object.assign(bannedSymbols, lugoConfig.get("diagnostics.bannedSymbols") || {});
	}

	ignoreGlobs = [...new Set(ignoreGlobs)];
	diagIgnoreGlobs = [...new Set(diagIgnoreGlobs)];
	libraryPaths = [...new Set(libraryPaths)];
	knownGlobals = [...new Set(knownGlobals)];

	return {
		libraryPaths: libraryPaths,
		ignoreGlobs: ignoreGlobs,
		diagIgnoreGlobs: diagIgnoreGlobs,
		knownGlobals: knownGlobals,
		bannedSymbols: bannedSymbols,
		maxFileSizeMB: primaryLugoConfig.get("workspace.maxFileSizeMB") ?? 4,

		parserMaxErrors: primaryLugoConfig.get("parser.maxErrors") ?? 50,

		diagUndefinedGlobals: primaryLugoConfig.get("diagnostics.undefinedGlobals") !== false,
		diagImplicitGlobals: primaryLugoConfig.get("diagnostics.implicitGlobals") !== false,
		diagUnusedLocal: primaryLugoConfig.get("diagnostics.unused.local") !== false,
		diagUnusedFunction: primaryLugoConfig.get("diagnostics.unused.function") !== false,
		diagUnusedParameter: primaryLugoConfig.get("diagnostics.unused.parameter") !== false,
		diagUnusedLoopVar: primaryLugoConfig.get("diagnostics.unused.loopVar") !== false,
		diagShadowing: primaryLugoConfig.get("diagnostics.shadowing") !== false,
		diagUnreachableCode: primaryLugoConfig.get("diagnostics.unreachableCode") !== false,
		diagAmbiguousReturns: primaryLugoConfig.get("diagnostics.ambiguousReturns") !== false,
		diagDeprecated: primaryLugoConfig.get("diagnostics.deprecated") !== false,
		diagDuplicateField: primaryLugoConfig.get("diagnostics.duplicateField") !== false,
		diagUnbalancedAssignment: primaryLugoConfig.get("diagnostics.unbalancedAssignment") !== false,
		diagDuplicateLocal: primaryLugoConfig.get("diagnostics.duplicateLocal") !== false,
		diagSelfAssignment: primaryLugoConfig.get("diagnostics.selfAssignment") !== false,
		diagEmptyBlock: primaryLugoConfig.get("diagnostics.emptyBlock") !== false,
		diagFormatString: primaryLugoConfig.get("diagnostics.formatString") !== false,
		diagTypeCheck: primaryLugoConfig.get("diagnostics.typeCheck") === true,
		diagRedundantParameter: primaryLugoConfig.get("diagnostics.redundantParameter") !== false,
		diagRedundantValue: primaryLugoConfig.get("diagnostics.redundantValue") !== false,
		diagRedundantReturn: primaryLugoConfig.get("diagnostics.redundantReturn") !== false,
		diagLoopVarMutation: primaryLugoConfig.get("diagnostics.loopVarMutation") !== false,
		diagIncorrectVararg: primaryLugoConfig.get("diagnostics.incorrectVararg") !== false,
		diagShadowingLoopVar: primaryLugoConfig.get("diagnostics.shadowingLoopVar") !== false,
		diagConstantCondition: primaryLugoConfig.get("diagnostics.constantCondition") !== false,
		diagUnreachableElse: primaryLugoConfig.get("diagnostics.unreachableElse") !== false,
		diagUsedIgnoredVar: primaryLugoConfig.get("diagnostics.usedIgnoredVariable") !== false,
		diagMinVariableNameLength: primaryLugoConfig.get("diagnostics.minVariableNameLength") ?? 0,
		diagIgnoredVariableNames: primaryLugoConfig.get("diagnostics.ignoredVariableNames") || ["_", "i", "j", "x", "y", "z", "w", "id", "to"],

		inlayParamHints: primaryLugoConfig.get("inlayHints.parameterNames") !== false,
		inlaySuppressMatch: primaryLugoConfig.get("inlayHints.suppressWhenArgumentMatchesName") !== false,
		inlayImplicitSelf: primaryLugoConfig.get("inlayHints.implicitSelf") !== false,

		featureDocHighlight: primaryLugoConfig.get("features.documentHighlight") !== false,
		featureHoverEval: primaryLugoConfig.get("features.hoverEvaluation") !== false,
		featureCodeLens: primaryLugoConfig.get("features.codeLens") !== false,
		featureFormatAlerts: primaryLugoConfig.get("features.formatAlerts") !== false,
		featureFormatting: primaryLugoConfig.get("features.formatting") !== false,
		formatOpinionated: primaryLugoConfig.get("features.formatOpinionated") === true,
		suggestFunctionParams: primaryLugoConfig.get("completion.suggestFunctionParams") !== false,

		featureFiveM: primaryLugoConfig.get("fivem.enabled") === true,
		diagFiveMUnaccountedFile: primaryLugoConfig.get("fivem.diagnostics.unaccountedFile") !== false,
		diagFiveMUnknownExport: primaryLugoConfig.get("fivem.diagnostics.unknownExport") !== false,
		diagFiveMUnknownResource: primaryLugoConfig.get("fivem.diagnostics.unknownResource") !== false,
	};
}

function scheduleConfigUpdate() {
	clearTimeout(debounce);

	debounce = setTimeout(() => {
		if (!client?.isRunning()) {
			return;
		}

		client.sendNotification("workspace/didChangeConfiguration", {
			settings: buildInitializationOptions(),
		});
	}, 1000);
}

async function activate(context) {
	const stdProvider = {
		provideTextDocumentContent: uri => {
			if (!client?.isRunning()) {
				return "";
			}

			return client
				.sendRequest("lugo/readStd", {
					uri: uri.toString(),
				})
				.then(res => {
					return res.content;
				});
		},
	};

	context.subscriptions.push(vscode.workspace.registerTextDocumentContentProvider("std", stdProvider));

	context.subscriptions.push(
		vscode.workspace.onDidChangeConfiguration(async e => {
			if (e.affectsConfiguration("lugo") || e.affectsConfiguration("files.exclude") || e.affectsConfiguration("search.exclude")) {
				scheduleConfigUpdate();
			}
		})
	);

	context.subscriptions.push(
		vscode.commands.registerCommand("lugo.reindex", () => {
			triggerReindex();
		})
	);

	context.subscriptions.push(
		vscode.commands.registerCommand("lugo.applySafeFixesWorkspace", () => {
			vscode.commands.executeCommand("lugo.applySafeFixes");
		})
	);

	context.subscriptions.push(
		vscode.commands.registerCommand("lugo.applySafeFixesFile", () => {
			const editor = vscode.window.activeTextEditor;

			if (editor) {
				vscode.commands.executeCommand("lugo.applySafeFixes", editor.document.uri.toString());
			}
		})
	);

	context.subscriptions.push(
		vscode.commands.registerCommand("lugo.ignoreDiagnostic", async (uriStr, line, rule, isFile) => {
			const editor = vscode.window.activeTextEditor;

			if (!editor || editor.document.uri.fsPath !== vscode.Uri.parse(uriStr).fsPath) {
				return;
			}

			let insertLine = line,
				snippetText = "";

			if (isFile) {
				insertLine = 0;
				snippetText = `---@diagnostic disable-file ${rule} - \${1:reason}\n`;
			} else {
				const targetLine = editor.document.lineAt(line),
					indent = targetLine.text.match(/^\s*/)[0];

				insertLine = line;
				snippetText = `${indent}---@diagnostic disable-next-line ${rule} - \${1:reason}\n`;
			}

			await editor.insertSnippet(new vscode.SnippetString(snippetText), new vscode.Position(insertLine, 0));
		})
	);

	context.subscriptions.push(
		vscode.commands.registerCommand("lugo.showReferences", (uriStr, position, locations) => {
			const uri = vscode.Uri.parse(uriStr),
				pos = new vscode.Position(position.line, position.character);

			const locs = locations.map(
				loc =>
					new vscode.Location(vscode.Uri.parse(loc.uri), new vscode.Range(loc.range.start.line, loc.range.start.character, loc.range.end.line, loc.range.end.character))
			);

			vscode.commands.executeCommand("editor.action.showReferences", uri, pos, locs);
		})
	);

	await restartClient(context);
}

async function startClient(context) {
	const initializationOptions = buildInitializationOptions();

	const platform = os.platform(),
		arch = os.arch(),
		ext = platform === "win32" ? ".exe" : "",
		binName = `lugo-${platform}-${arch}${ext}`;

	const serverCommand = path.join(context.extensionPath, "bin", binName);

	if (!fs.existsSync(serverCommand)) {
		vscode.window.showErrorMessage(`Lugo LSP binary not found for your platform: ${binName}`);

		return;
	}

	const serverOptions = {
		run: { command: serverCommand },
		debug: { command: serverCommand },
	};

	const clientOptions = {
		documentSelector: [
			{ scheme: "file", language: "lua" },
			{ scheme: "untitled", language: "lua" },
			{ scheme: "std", language: "lua" },
		],
		synchronize: {
			fileEvents: vscode.workspace.createFileSystemWatcher("**/*.lua"),
		},
		initializationOptions: initializationOptions,
	};

	client = new LanguageClient("lugo", "Lugo LSP", serverOptions, clientOptions);

	await client.start();

	triggerReindex();
}

function triggerReindex() {
	if (!client || indexing) {
		return;
	}

	indexing = true;

	vscode.window.withProgress(
		{
			location: vscode.ProgressLocation.Window,
			title: "Lugo: Indexing workspace...",
			cancellable: false,
		},
		async () => {
			try {
				await client.sendRequest("lugo/reindex");
			} finally {
				indexing = false;
			}
		}
	);
}

function deactivate() {
	if (debounce) {
		clearTimeout(debounce);
	}

	if (!client) {
		return undefined;
	}

	return client.stop();
}

module.exports = {
	activate: activate,
	deactivate: deactivate,
};

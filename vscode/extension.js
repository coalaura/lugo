const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const vscode = require("vscode");
const { LanguageClient } = require("vscode-languageclient/node");

let client, restarting, indexing, debounce;

async function restartClient(context) {
	if (restarting) {
		scheduleClientRestart(context);

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

function scheduleClientRestart(context) {
	clearTimeout(debounce);

	debounce = setTimeout(() => {
		restartClient(context);
	}, 1000);
}

async function activate(context) {
	await restartClient(context);

	const stdProvider = {
		provideTextDocumentContent: uri => {
			if (!client) {
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
		vscode.workspace.onDidChangeConfiguration(e => {
			if (e.affectsConfiguration("lugo")) {
				scheduleClientRestart(context);
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
}

async function startClient(context) {
	const config = vscode.workspace.getConfiguration("lugo"),
		libraryPaths = config.get("workspace.libraryPaths") || [],
		knownGlobals = config.get("environment.knownGlobals") || [];

	const diagUndefinedGlobals = config.get("diagnostics.undefinedGlobals") !== false,
		diagImplicitGlobals = config.get("diagnostics.implicitGlobals") !== false,
		diagUnusedLocal = config.get("diagnostics.unusedLocal") !== false,
		diagUnusedFunction = config.get("diagnostics.unusedFunction") !== false,
		diagUnusedParameter = config.get("diagnostics.unusedParameter") !== false,
		diagUnusedLoopVar = config.get("diagnostics.unusedLoopVar") !== false,
		diagShadowing = config.get("diagnostics.shadowing") !== false,
		diagUnreachableCode = config.get("diagnostics.unreachableCode") !== false,
		diagAmbiguousReturns = config.get("diagnostics.ambiguousReturns") !== false,
		diagDeprecated = config.get("diagnostics.deprecated") !== false,
		inlayParamHints = config.get("inlayHints.parameterNames") !== false,
		inlaySuppressMatch = config.get("inlayHints.suppressWhenArgumentMatchesName") !== false,
		featureDocumentHighlight = config.get("features.documentHighlight") !== false,
		featureHoverEvaluation = config.get("features.hoverEvaluation") !== false,
		featureCodeLens = config.get("features.codeLens") !== false;

	const filesConfig = vscode.workspace.getConfiguration("files"),
		searchConfig = vscode.workspace.getConfiguration("search");

	let ignoreGlobs = config.get("workspace.ignoreGlobs") || [];

	const nativeExcludes = {
		...(filesConfig.get("exclude") || {}),
		...(searchConfig.get("exclude") || {}),
	};

	for (const [key, val] of Object.entries(nativeExcludes)) {
		if (val === true) {
			ignoreGlobs.push(key);
		}
	}

	ignoreGlobs = [...new Set(ignoreGlobs)];

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
			{ scheme: "std", language: "lua" },
		],
		initializationOptions: {
			libraryPaths: libraryPaths,
			knownGlobals: knownGlobals,
			ignoreGlobs: ignoreGlobs,

			diagnosticsUndefinedGlobals: diagUndefinedGlobals,
			diagnosticsImplicitGlobals: diagImplicitGlobals,
			diagnosticsUnusedLocal: diagUnusedLocal,
			diagnosticsUnusedFunction: diagUnusedFunction,
			diagnosticsUnusedParameter: diagUnusedParameter,
			diagnosticsUnusedLoopVar: diagUnusedLoopVar,
			diagnosticsShadowing: diagShadowing,
			diagnosticsUnreachableCode: diagUnreachableCode,
			diagnosticsAmbiguousReturns: diagAmbiguousReturns,
			diagnosticsDeprecated: diagDeprecated,
			inlayHintsParameterNames: inlayParamHints,
			inlayHintsSuppressWhenArgumentMatchesName: inlaySuppressMatch,
			featuresDocumentHighlight: featureDocumentHighlight,
			featuresHoverEvaluation: featureHoverEvaluation,
			featuresCodeLens: featureCodeLens,
		},
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
			await client.sendRequest("lugo/reindex");

			indexing = false;
		}
	);
}

function deactivate() {
	clearTimeout(debounce);

	if (!client) {
		return;
	}

	return client.stop();
}

module.exports = {
	activate: activate,
	deactivate: deactivate,
};

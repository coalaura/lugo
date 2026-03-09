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
	const config = vscode.workspace.getConfiguration("lugo");

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
		synchronize: {
			fileEvents: vscode.workspace.createFileSystemWatcher("**/*.lua"),
		},
		initializationOptions: {
			libraryPaths: config.get("workspace.libraryPaths") || [],
			ignoreGlobs: ignoreGlobs,
			knownGlobals: config.get("environment.knownGlobals") || [],

			parserMaxErrors: config.get("parser.maxErrors") ?? 50,

			diagUndefinedGlobals: config.get("diagnostics.undefinedGlobals") !== false,
			diagImplicitGlobals: config.get("diagnostics.implicitGlobals") !== false,
			diagUnusedLocal: config.get("diagnostics.unused.local") !== false,
			diagUnusedFunction: config.get("diagnostics.unused.function") !== false,
			diagUnusedParameter: config.get("diagnostics.unused.parameter") !== false,
			diagUnusedLoopVar: config.get("diagnostics.unused.loopVar") !== false,
			diagShadowing: config.get("diagnostics.shadowing") !== false,
			diagUnreachableCode: config.get("diagnostics.unreachableCode") !== false,
			diagAmbiguousReturns: config.get("diagnostics.ambiguousReturns") !== false,
			diagDeprecated: config.get("diagnostics.deprecated") !== false,
			diagDuplicateField: config.get("diagnostics.duplicateField") !== false,
			diagUnbalancedAssignment: config.get("diagnostics.unbalancedAssignment") !== false,
			diagDuplicateLocal: config.get("diagnostics.duplicateLocal") !== false,
			diagSelfAssignment: config.get("diagnostics.selfAssignment") !== false,
			diagEmptyBlock: config.get("diagnostics.emptyBlock") !== false,
			diagTypeCheck: config.get("diagnostics.typeCheck") === true,

			inlayParamHints: config.get("inlayHints.parameterNames") !== false,
			inlaySuppressMatch: config.get("inlayHints.suppressWhenArgumentMatchesName") !== false,

			featureDocHighlight: config.get("features.documentHighlight") !== false,
			featureHoverEval: config.get("features.hoverEvaluation") !== false,
			featureCodeLens: config.get("features.codeLens") !== false,
			featureFormatting: config.get("features.formatting") !== false,
			formatOpinionated: config.get("features.formatOpinionated") === true,
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

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
	const filesConfig = vscode.workspace.getConfiguration("files"),
		searchConfig = vscode.workspace.getConfiguration("search"),
		lugoConfig = vscode.workspace.getConfiguration("lugo");

	let ignoreGlobs = lugoConfig.get("workspace.ignoreGlobs") || [];

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

	return {
		libraryPaths: lugoConfig.get("workspace.libraryPaths") || [],
		ignoreGlobs: ignoreGlobs,
		knownGlobals: lugoConfig.get("environment.knownGlobals") || [],

		parserMaxErrors: lugoConfig.get("parser.maxErrors") ?? 50,

		diagUndefinedGlobals: lugoConfig.get("diagnostics.undefinedGlobals") !== false,
		diagImplicitGlobals: lugoConfig.get("diagnostics.implicitGlobals") !== false,
		diagUnusedLocal: lugoConfig.get("diagnostics.unused.local") !== false,
		diagUnusedFunction: lugoConfig.get("diagnostics.unused.function") !== false,
		diagUnusedParameter: lugoConfig.get("diagnostics.unused.parameter") !== false,
		diagUnusedLoopVar: lugoConfig.get("diagnostics.unused.loopVar") !== false,
		diagShadowing: lugoConfig.get("diagnostics.shadowing") !== false,
		diagUnreachableCode: lugoConfig.get("diagnostics.unreachableCode") !== false,
		diagAmbiguousReturns: lugoConfig.get("diagnostics.ambiguousReturns") !== false,
		diagDeprecated: lugoConfig.get("diagnostics.deprecated") !== false,
		diagDuplicateField: lugoConfig.get("diagnostics.duplicateField") !== false,
		diagUnbalancedAssignment: lugoConfig.get("diagnostics.unbalancedAssignment") !== false,
		diagDuplicateLocal: lugoConfig.get("diagnostics.duplicateLocal") !== false,
		diagSelfAssignment: lugoConfig.get("diagnostics.selfAssignment") !== false,
		diagEmptyBlock: lugoConfig.get("diagnostics.emptyBlock") !== false,
		diagFormatString: lugoConfig.get("diagnostics.formatString") !== false,
		diagTypeCheck: lugoConfig.get("diagnostics.typeCheck") === true,
		diagRedundantParameter: lugoConfig.get("diagnostics.redundantParameter") !== false,
		diagRedundantValue: lugoConfig.get("diagnostics.redundantValue") !== false,
		diagRedundantReturn: lugoConfig.get("diagnostics.redundantReturn") !== false,
		diagLoopVarMutation: lugoConfig.get("diagnostics.loopVarMutation") !== false,
		diagIncorrectVararg: lugoConfig.get("diagnostics.incorrectVararg") !== false,
		diagShadowingLoopVar: lugoConfig.get("diagnostics.shadowingLoopVar") !== false,
		diagUnreachableElse: lugoConfig.get("diagnostics.unreachableElse") !== false,

		inlayParamHints: lugoConfig.get("inlayHints.parameterNames") !== false,
		inlaySuppressMatch: lugoConfig.get("inlayHints.suppressWhenArgumentMatchesName") !== false,
		inlayImplicitSelf: lugoConfig.get("inlayHints.implicitSelf") !== false,

		featureDocHighlight: lugoConfig.get("features.documentHighlight") !== false,
		featureHoverEval: lugoConfig.get("features.hoverEvaluation") !== false,
		featureCodeLens: lugoConfig.get("features.codeLens") !== false,
		featureFormatting: lugoConfig.get("features.formatting") !== false,
		formatOpinionated: lugoConfig.get("features.formatOpinionated") === true,
		suggestFunctionParams: lugoConfig.get("completion.suggestFunctionParams") !== false,
	};
}

function scheduleConfigUpdate() {
	clearTimeout(debounce);

	debounce = setTimeout(() => {
		if (!client || !client.isRunning()) {
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
			if (!client || !client.isRunning()) {
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

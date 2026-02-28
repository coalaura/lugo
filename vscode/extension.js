const fs = require("fs");
const os = require("os");
const path = require("path");
const vscode = require("vscode");
const { LanguageClient } = require("vscode-languageclient/node");

let client;

async function activate(context) {
	await startClient(context);

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
		vscode.workspace.onDidChangeConfiguration(async e => {
			if (e.affectsConfiguration("lugo")) {
				if (client) {
					await client.stop();
				}

				await startClient(context);
			}
		})
	);

	context.subscriptions.push(
		vscode.commands.registerCommand("lugo.reindex", () => {
			triggerReindex();
		})
	);
}

async function startClient(context) {
	const config = vscode.workspace.getConfiguration("lugo"),
		libraryPaths = config.get("workspace.libraryPaths") || [],
		knownGlobals = config.get("environment.knownGlobals") || [];

	const diagUndefinedGlobals = config.get("diagnostics.undefinedGlobals") !== false,
		diagImplicitGlobals = config.get("diagnostics.implicitGlobals") !== false,
		diagUnusedVariables = config.get("diagnostics.unusedVariables") !== false,
		diagShadowing = config.get("diagnostics.shadowing") !== false,
		diagUnreachableCode = config.get("diagnostics.unreachableCode") !== false,
		diagAmbiguousReturns = config.get("diagnostics.ambiguousReturns") !== false,
		diagDeprecated = config.get("diagnostics.deprecated") !== false,
		inlayParamHints = config.get("inlayHints.parameterNames") !== false;

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
			diagnosticsUnusedVariables: diagUnusedVariables,
			diagnosticsShadowing: diagShadowing,
			diagnosticsUnreachableCode: diagUnreachableCode,
			diagnosticsAmbiguousReturns: diagAmbiguousReturns,
			diagnosticsDeprecated: diagDeprecated,
			inlayHintsParameterNames: inlayParamHints,
		},
	};

	client = new LanguageClient("lugo", "Lugo LSP", serverOptions, clientOptions);

	await client.start();

	triggerReindex();
}

function triggerReindex() {
	if (!client) {
		return;
	}

	vscode.window.withProgress(
		{
			location: vscode.ProgressLocation.Window,
			title: "Lugo: Indexing workspace...",
			cancellable: false,
		},
		async () => {
			await client.sendRequest("lugo/reindex");
		}
	);
}

function deactivate() {
	if (!client) {
		return;
	}

	return client.stop();
}

module.exports = {
	activate: activate,
	deactivate: deactivate,
};

local lspconfig = require("lspconfig")
local configs = require("lspconfig.configs")

-- Define the custom Lugo server
if not configs.lugo then
	configs.lugo = {
		default_config = {
			cmd = {
				"/path/to/your/lugo-linux-amd64" -- Update this path
			},
			filetypes = { "lua" },
			root_dir = lspconfig.util.root_pattern(".git", ".luarc.json"),
			settings = {}
		}
	}
end

-- Setup and pass initialization options directly
lspconfig.lugo.setup({
	init_options = {
		libraryPaths = {},
		ignoreGlobs = {
			"**/node_modules/**",
			"**/.git/**"
		},
		knownGlobals = {
			"vim"
		},
		bannedSymbols = {},
		maxFileSizeMB = 4,

		-- Parser
		parserMaxErrors = 50,

		-- Diagnostics
		diagUndefinedGlobals = true,
		diagImplicitGlobals = true,
		diagUnusedLocal = true,
		diagUnusedFunction = true,
		diagUnusedParameter = true,
		diagUnusedLoopVar = true,
		diagShadowing = true,
		diagUnreachableCode = true,
		diagAmbiguousReturns = true,
		diagDeprecated = true,
		diagDuplicateField = true,
		diagUnbalancedAssignment = true,
		diagDuplicateLocal = true,
		diagSelfAssignment = true,
		diagEmptyBlock = true,
		diagFormatString = true,
		diagTypeCheck = false, -- Set to true if using strict LuaCATS annotations
		diagRedundantParameter = true,
		diagRedundantValue = true,
		diagRedundantReturn = true,
		diagLoopVarMutation = true,
		diagIncorrectVararg = true,
		diagShadowingLoopVar = true,
		diagConstantCondition = true,
		diagUnreachableElse = true,
		diagUsedIgnoredVar = true,

		-- Inlay Hints
		inlayParamHints = true,
		inlaySuppressMatch = true,
		inlayImplicitSelf = true,

		-- Editor Features
		featureDocHighlight = true,
		featureHoverEval = true,
		featureCodeLens = true,
		featureFormatting = true,
		formatOpinionated = false,
		suggestFunctionParams = true,
		featureFormatAlerts = true,

		-- FiveM Support
		featureFiveM = false, -- Set to true if working on FiveM resources
		diagFiveMUnaccountedFile = true,
		diagFiveMUnknownExport = true,
		diagFiveMUnknownResource = true
	}
})

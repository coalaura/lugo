![banner](banner.jpg)

[![Tests](https://github.com/coalaura/lugo/actions/workflows/test.yml/badge.svg)](https://github.com/coalaura/lugo/actions/workflows/test.yml)

A ridiculously fast, zero-allocation Lua 5.4 parser and Language Server (LSP) written in Go.

Lugo is built from the ground up for maximum performance. By iterating over source code using a flat-array/arena architecture (`[]Node`) and storing only byte offsets, it heavily eliminates pointer allocations, heap strings and garbage collection pressure.

[**Install from the VS Code Marketplace**](https://marketplace.visualstudio.com/items?itemName=coalaura.lugo-vscode)

## Why Lugo?

Most Lua language servers struggle when dropped into massive codebases (like game server environments or large modding frameworks). They consume gigabytes of RAM, take minutes to index and lag while typing.

**Lugo is different:**
* **Blistering Performance:** In real-world benchmarks on modern hardware, Lugo completely cold-indexes massive workspaces (including full AST generation, symbol resolution and publishing workspace-wide diagnostics) in a matter of seconds.
  ```text
  2026-04-06T23:06:52 Starting workspace re-index...
  2026-04-06T23:06:52 Indexing external library: /opt/lua-fivem-sdk
  2026-04-06T23:06:52 Indexing workspace folder: file:///workspace/server/resources
  2026-04-06T23:06:54 Indexing workspace folder: file:///workspace/server/framework-assets
  2026-04-06T23:06:54 Indexing workspace folder: file:///workspace/server/legacy-assets
  2026-04-06T23:06:54 Re-indexed workspace in 1.9606123s (indexed=3020, unchanged=1, failed=0)
  2026-04-06T23:06:55 Published diagnostics for 2997 files in 487.4179ms
  2026-04-06T23:06:55 Total time taken for 28619407 bytes: 2.4480302s
  ```
* **Incremental Warm Starts:** Lugo hashes your workspace files. If you trigger a re-index, it skips parsing unchanged files and reuses map memory pools (`clear()`), dropping warm re-indexes to a fraction of a second.
* **Zero-Allocation Architecture:** The parser, lexer and symbol resolver are designed to never allocate heap strings during normal typing. Tight loops execute inside CPU registers, leveraging SIMD-accelerated byte scanning to maximize cache locality.
* **Microscopic Memory Footprint:** Lugo only stores flat arrays of integers. Only actively open files keep their source strings in memory, meaning Lugo can index thousands of files while consuming a fraction of the RAM used by traditional LSPs.
* **Dynamic by Design:** Instead of forcing strict typing on a dynamic language, Lugo embraces Lua. If you do `MySQL = this` in a local file, Lugo dynamically resolves all deep table fields (e.g., `MySQL.Await.Execute`) across your entire workspace in real-time.
* **Standalone Binary:** No NodeJS, no Java, no Lua runtimes. Just a single, blazingly fast compiled Go binary.

## Features & Capabilities

Lugo implements a comprehensive suite of modern Language Server Protocol features:

* **Intelligent Autocomplete:** Resilient, context-aware member access (`table.|`), locals, globals and keywords. Works even when the surrounding syntax tree is temporarily broken.
* **Semantic Tokens (Rich Highlighting):** Compiler-accurate syntax highlighting. Visually distinguishes locals from globals, properties from methods and identifies modifiers like `readonly` (`<const>`), `deprecated` and `defaultLibrary`.
* **Document Highlights:** Click or move your cursor over any variable or function to instantly highlight all read/write usages within the current file.
* **Smart Selection (Selection Range):** Press `Shift+Alt+RightArrow` to semantically expand your text selection based on the AST (Identifier -> Call Expression -> Statement -> Block -> Function -> File).
* **Go to Definition & Hover:** Instant cross-file jumps. Fully parses LuaDoc (`@param`, `@return`, `@field`, `@class`, `@alias`, `@type`, `@generic`, `@overload`, `@see`, `@deprecated`) and renders beautifully formatted function signatures.
* **Hover Evaluation:** Lugo statically evaluates constant expressions (math, bitwise operations, string concatenation and logic) in real-time, displaying the computed result directly in the hover tooltip.
* **Advanced Type Inference:** Lazily evaluates and caches types. Supports control-flow type narrowing (e.g., `type(x) == "string"`), loop variable unpacking (`ipairs`/`pairs`), `require` module aliasing/exports and deep metatable resolution (understands `setmetatable` and `__index` inheritance).
* **Find References & Code Lens:** Find all usages of a symbol across your workspace. Automatically embeds clickable Code Lens reference counters directly above function definitions.
* **Format Alerts:** Automatically formats special comment tags (e.g., `NOTE:`, `TODO:`, `FIXME:`, `WARNING:`) with emojis and bold text in hover tooltips for better visibility.
* **Rename & Linked Editing Ranges:** Instantly rename symbols across your workspace. Supports Linked Editing for simultaneous, multi-cursor renaming of local variables as you type.
* **Call Hierarchy:** Visually explore a tree of incoming and outgoing function calls.
* **Document & Workspace Symbols:** Instant workspace-wide search (`Ctrl+T`) for fully qualified names (e.g., `OP.Math.Round`) and full VS Code "Outline" tree generation.
* **Signature Help & Inlay Hints:** Real-time active-parameter tooltips and inline parameter name hints with smart implicit `self` offset calculation. Automatically suppresses hints when the argument matches the parameter name to reduce visual noise.
* **Code Actions (Quick Fixes & Refactoring):** Fast automated fixes for common diagnostics (prefixing unused variables, adding `local`, fixing typos). Includes powerful **AST-aware refactorings**: invert conditions, recursively convert `if` chains to early returns, optimize `table.insert` to `t[#t+1]`, convert between dot/colon method signatures, merge nested `if` statements, split multiple assignments, swap `if`/`else` branches, remove redundant parentheses, convert `for i=1, #t` to `ipairs` and toggle between dot/bracket table indexing. Includes **bulk Safe Fixes** (via command palette) to automatically clean up unused variables, parameters and assignments across the current file or your entire workspace securely without breaking side-effects.
* **Full Lua 5.4 Support:** Native parsing, type-inference and semantic highlighting for `<const>` and `<close>` attributes, `goto` statements and `::labels::`.
* **FiveM Resource Isolation:** Native support for parsing `fxmanifest.lua` and `__resource.lua`. Lugo automatically isolates `client`, `server` and `shared` environments, preventing cross-contamination of globals and providing warnings if a file isn't referenced in the manifest.
* **File Watching:** Automatically synchronizes with workspace file creations, deletions and external changes in real-time.
* **Built-in Formatter:** A blazingly fast, AST-aware Lua formatter. Elegantly fixes whitespace, enforces indentation rules, strips trailing semicolons, expands minified code and optionally applies opinionated stylistic tweaks (like separating unrelated statements with blank lines).
* **Folding Ranges:** Accurately fold functions, tables, control flow blocks and multi-line strings/comments.
* **Virtual Standard Library:** Click on any standard library function to open a syntax-highlighted, read-only virtual tab streaming directly from the Go server's embedded filesystem (`std:///`).
* **Fast-Path Smart Ignores:** Automatically inherits VS Code's native `files.exclude` and `search.exclude` settings. Lugo pre-compiles these into high-speed prefix/suffix byte matchers, instantly skipping ignored directories without the overhead of regex.

### Advanced Diagnostics
Lugo performs workspace-wide analysis to catch bugs before runtime:
* **Undefined Globals:** Detects typos with wildcard ignore support (e.g., `N_0x*`) and provides quick-fixes to the closest known global.
* **Implicit Globals:** Warns when you forget the `local` keyword inside a function and provides a quick-fix to inject it.
* **Unused Variables:** Granular detection for unused locals, functions, parameters and loop variables.
* **Shadowing:** Warns when a local or loop variable shadows an outer scope or global, providing a clickable link to the shadowed definition.
* **Unreachable Code:** Detects dead code after `return`, `break` or `goto`, as well as statically unreachable `elseif` or `else` branches.
* **Ambiguous Returns:** Catches Lua's infamous newline evaluation trap where expressions on the next line are accidentally returned.
* **Redundant Code:** Warns about empty blocks (`do end`), self-assignments, redundant parameters, redundant assignment values and redundant returns (with quick-fixes to remove them).
* **Sanity Checks:** Detects duplicate fields in table literals, unbalanced assignments, loop variable mutations and incorrect vararg (`...`) usage.
* **Type Checking:** Optionally catches strictly invalid operations like attempting to call a number or index a non-table.
* **Format String Validation:** Warns when `string.format` is called with an incorrect number of arguments.
* **Used Ignored Variables:** Warns when a variable conventionally marked as ignored (prefixed with `_`) is actually used in the code, offering a quick-fix to safely rename it.
* **Deprecation:** Warns when using symbols marked with `@deprecated`.

## FiveM Support (Optional)

Lugo includes first-class, built-in support for FiveM resource development. When enabled via the `lugo.fivem.enabled` setting, Lugo will automatically parse `fxmanifest.lua` and `__resource.lua` files to accurately map your project structure, enabling resource-aware completions and diagnostics.

* **Environment Isolation:** Automatically detects whether a file is `client`, `server` or `shared`. Client files cannot see server-only globals and vice versa.
* **Resource Scoping:** Globals defined in one resource will not leak into another resource.
* **Cross-Resource Includes:** Understands `@resource_name/file.lua` syntax in manifests for cross-resource dependencies.
* **Unaccounted File Warnings:** Warns you if a `.lua` file exists in your workspace but is missing from the resource manifest, preventing "script not running" headaches.
* **Export Validation:** Validates `exports.resource_name:methodName()` cross-resource calls, warning you if the resource or the specific export is unknown.

## Installation

### VS Code
Simply install the extension from the [VS Code Marketplace](https://marketplace.visualstudio.com/items?itemName=coalaura.lugo-vscode). The extension automatically detects your OS and architecture and runs the correct bundled Go binary. No external dependencies are required.

### Other Editors (Neovim, Helix, etc.)
Lugo is entirely editor-agnostic and communicates using standard JSON-RPC over `stdio`. You can download the standalone LSP binaries for Windows, Linux and macOS from the [GitHub Releases](https://github.com/coalaura/lugo/releases) page.

Because Lugo does not rely on a generic wrapper, you must pass your settings directly into `initializationOptions` when setting up the client.

#### Neovim (`nvim-lspconfig`)
You can easily add Lugo as a custom server in your Neovim environment. Since Lugo is standalone, you will need to pass the initialization options directly.

See [**`example.init.lua`**](example.init.lua) for a complete setup snippet.

## CI/CD Pipeline Integration

Lugo can be run directly in your CI/CD pipelines (like GitHub Actions) to enforce the exact same strictness and diagnostics as your local editor. By passing the `--ci` flag along with a configuration JSON file, Lugo bypasses the standard JSON-RPC loop, indexes your workspace and outputs diagnostics in standard GitHub Actions format (`::warning`, `::error`).

This means line-specific annotations will automatically appear in your Pull Request diffs!

Check out the examples:
* [**`example.ci.json`**](example.ci.json) - An example CI configuration file (maps exactly to the LSP `initializationOptions`).
* [**`example.ci.yml`**](example.ci.yml) - A sample GitHub Actions workflow demonstrating how to download and execute Lugo.

## Configuration

You can configure Lugo via your VS Code `settings.json` (also available via the settings UI under **Extensions -> Lugo LSP**):

**Workspace & Environment**
* `lugo.workspace.libraryPaths`: An array of absolute paths to external Lua libraries to index.
* `lugo.workspace.ignoreGlobs`: Additional glob patterns to ignore during indexing. Inherits VS Code's `files.exclude` automatically.
* `lugo.environment.knownGlobals`: Global variables to ignore when reporting undefined globals. Supports wildcards (e.g., `N_0x*`).

**Parser & Diagnostics**
* `lugo.parser.maxErrors`: Maximum number of syntax errors to report per file (default: `50`). Reduces cascade noise on heavily broken files. Set to `0` for unlimited.
* `lugo.diagnostics.undefinedGlobals`: Toggle undefined global warnings.
* `lugo.diagnostics.implicitGlobals`: Toggle warnings for forgetting the `local` keyword.
* `lugo.diagnostics.unused.local`: Toggle unused local variable detection.
* `lugo.diagnostics.unused.function`: Toggle unused local function detection.
* `lugo.diagnostics.unused.parameter`: Toggle unused parameter detection.
* `lugo.diagnostics.unused.loopVar`: Toggle unused loop variable detection.
* `lugo.diagnostics.shadowing`: Toggle warnings when a local shadows an outer scope or global.
* `lugo.diagnostics.unreachableCode`: Toggle graying out unreachable code.
* `lugo.diagnostics.ambiguousReturns`: Toggle warnings for expressions accidentally returned due to newlines.
* `lugo.diagnostics.duplicateField`: Toggle warnings for duplicate fields inside table literals.
* `lugo.diagnostics.unbalancedAssignment`: Toggle warnings when assigning fewer or more values than variables.
* `lugo.diagnostics.duplicateLocal`: Toggle warnings when a local variable is defined twice in the exact same scope.
* `lugo.diagnostics.selfAssignment`: Toggle warnings when assigning a variable to itself.
* `lugo.diagnostics.emptyBlock`: Toggle hints for empty blocks (e.g., `do end`).
* `lugo.diagnostics.formatString`: Toggle diagnostics for `string.format` argument counts.
* `lugo.diagnostics.typeCheck`: Toggle strict type checking for operations like calling numbers or indexing non-tables.
* `lugo.diagnostics.redundantParameter`: Toggle diagnostics for passing more arguments to a function than it accepts.
* `lugo.diagnostics.redundantValue`: Toggle diagnostics for assigning more values than there are variables.
* `lugo.diagnostics.redundantReturn`: Toggle diagnostics for empty return statements at the very end of a function.
* `lugo.diagnostics.loopVarMutation`: Toggle diagnostics for mutating a loop variable inside the loop body.
* `lugo.diagnostics.incorrectVararg`: Toggle diagnostics for using the vararg `...` expression outside of a vararg function.
* `lugo.diagnostics.shadowingLoopVar`: Toggle diagnostics when a loop variable shadows an outer local or global variable.
* `lugo.diagnostics.unreachableElse`: Toggle diagnostics for unreachable `elseif` or `else` branches.
* `lugo.diagnostics.usedIgnoredVariable`: Toggle diagnostics for variables that are used but their name starts with `_`.
* `lugo.diagnostics.deprecated`: Toggle warnings for usage of `@deprecated` symbols.

**Editor Features**
* `lugo.completion.suggestFunctionParams`: Automatically insert function parameters as snippets when autocompleting a function call.
* `lugo.inlayHints.parameterNames`: Enable inline parameter name hints for function and method calls.
* `lugo.inlayHints.suppressWhenArgumentMatchesName`: Suppress parameter name hints when the argument name exactly matches the parameter name (e.g., avoiding `pSource: pSource`).
* `lugo.inlayHints.implicitSelf`: Enable inline `self` hints for method definitions using the colon syntax.
* `lugo.features.documentHighlight`: Enable document highlights for variables and function/method calls.
* `lugo.features.hoverEvaluation`: Evaluate and display the result of constant expressions on hover (e.g., `1 + 2` -> `3`).
* `lugo.features.codeLens`: Enable CodeLens annotations (e.g., reference counts) above function definitions.
* `lugo.features.formatAlerts`: Format special comment tags (e.g., `NOTE:`, `TODO:`) with emojis and bold text in hovers.
* `lugo.features.formatting`: Enable the built-in Lua formatter for inline format fixing and document formatting.
* `lugo.features.formatOpinionated`: Apply opinionated formatting tweaks (e.g., forcing blank lines between unrelated statements).

**FiveM Support**
* `lugo.fivem.enabled`: Enable FiveM support. Scopes globals and diagnostics to their respective resources (using `fxmanifest.lua` / `__resource.lua`).
* `lugo.fivem.diagnostics.unaccountedFile`: Toggle warnings for files missing from the resource manifest.
* `lugo.fivem.diagnostics.unknownExport`: Toggle warnings for calling missing exports on a FiveM resource.
* `lugo.fivem.diagnostics.unknownResource`: Toggle warnings for referencing unknown FiveM resources via the `exports` global.

## Commands

Available via the VS Code Command Palette (`Ctrl+Shift+P`):
* **Lugo: Re-index Workspace:** Manually trigger a full workspace re-index.
* **Lugo: Apply Safe Fixes (Current File):** Automatically clean up unused variables, parameters and assignments in the active file without breaking side-effects.
* **Lugo: Apply Safe Fixes (Workspace):** Apply all safe fixes across the entire workspace.

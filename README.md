# Lugo

[![Tests](https://github.com/coalaura/lugo/actions/workflows/test.yml/badge.svg)](https://github.com/coalaura/lugo/actions/workflows/test.yml)

A ridiculously fast, zero-allocation Lua 5.4 parser and Language Server (LSP) written in Go. 

Lugo is built from the ground up for maximum performance. By iterating over source code using a flat-array/arena architecture (`[]Node`) and storing only byte offsets, it heavily eliminates pointer allocations, heap strings, and garbage collection pressure.

[**Install from the VS Code Marketplace**](https://marketplace.visualstudio.com/items?itemName=coalaura.lugo-vscode)

## Why Lugo?

Most Lua language servers struggle when dropped into massive codebases (like game server environments or large modding frameworks). They consume gigabytes of RAM, take minutes to index, and lag while typing.

**Lugo is different:**
* **Blistering Performance:** In real-world benchmarks on modern hardware (e.g., AMD Ryzen 9 9950X3D), Lugo completely cold-indexes a **2,200+ file workspace** (including full AST generation, resolution, and publishing workspace-wide diagnostics) in **~1.5s**.
* **Incremental Warm Starts:** Lugo hashes your workspace files. If you restart the server or trigger a re-index, it skips parsing unchanged files and reuses map memory pools (`clear()`), dropping warm re-indexes to just **~340ms**.
* **Zero-Allocation Architecture:** The parser, lexer, and symbol resolver are designed to never allocate heap strings during normal typing. Tight loops execute inside CPU registers, leveraging SIMD-accelerated byte scanning to maximize cache locality.
* **Microscopic Memory Footprint:** Lugo only stores flat arrays of integers. Only actively open files keep their source strings in memory, meaning Lugo can index thousands of files while consuming a fraction of the RAM used by traditional LSPs.
* **Dynamic by Design:** Instead of forcing strict typing on a dynamic language, Lugo embraces Lua. If you do `MySQL = this` in a local file, Lugo dynamically resolves all deep table fields (e.g., `MySQL.Await.Execute`) across your entire workspace in real-time.
* **Standalone Binary:** No NodeJS, no Java, no Lua runtimes. Just a single, blazingly fast compiled Go binary.

## Features & Capabilities

Lugo implements a comprehensive suite of modern Language Server Protocol features:

* **Intelligent Autocomplete:** Resilient, context-aware member access (`table.|`), locals, globals, and keywords. Works even when the surrounding syntax tree is temporarily broken.
* **Semantic Tokens (Rich Highlighting):** Compiler-accurate syntax highlighting. Visually distinguishes locals from globals, properties from methods, and identifies modifiers like `readonly` (`<const>`), `deprecated`, and `defaultLibrary`.
* **Document Highlights:** Click or move your cursor over any variable or function to instantly highlight all read/write usages within the current file.
* **Smart Selection (Selection Range):** Press `Shift+Alt+RightArrow` to semantically expand your text selection based on the AST (Identifier -> Call Expression -> Statement -> Block -> Function -> File).
* **Go to Definition & Hover:** Instant cross-file jumps. Fully parses LuaDoc (`@param`, `@return`, `@field`, `@class`, `@alias`, `@type`, `@generic`, `@overload`, `@see`, `@deprecated`) and renders beautifully formatted function signatures.
* **Find References & Code Lens:** Find all usages of a symbol across your workspace. Automatically embeds clickable Code Lens reference counters directly above function definitions.
* **Rename & Linked Editing Ranges:** Instantly rename symbols across your workspace. Supports Linked Editing for simultaneous, multi-cursor renaming of local variables as you type.
* **Call Hierarchy:** Visually explore a tree of incoming and outgoing function calls.
* **Document & Workspace Symbols:** Instant workspace-wide search (`Ctrl+T`) for fully qualified names (e.g., `OP.Math.Round`), and full VS Code "Outline" tree generation.
* **Signature Help & Inlay Hints:** Real-time active-parameter tooltips and inline parameter name hints with smart implicit `self` offset calculation.
* **Code Actions (Quick Fixes):** Fast automated fixes for common diagnostics, including prefixing unused variables with `_`, adding `local` to implicit globals, and instantly fixing typos with highly optimized Levenshtein distance suggestions.
* **Folding Ranges:** Accurately fold functions, tables, control flow blocks, and multi-line strings/comments.
* **Virtual Standard Library:** Click on any standard library function to open a syntax-highlighted, read-only virtual tab streaming directly from the Go server's embedded filesystem (`std:///`).
* **Fast-Path Smart Ignores:** Automatically inherits VS Code's native `files.exclude` and `search.exclude` settings. Lugo pre-compiles these into high-speed prefix/suffix byte matchers, instantly skipping ignored directories without the overhead of regex.

### Advanced Diagnostics
Lugo performs workspace-wide analysis to catch bugs before runtime:
* **Undefined Globals:** Detects typos with wildcard ignore support (e.g., `N_0x*`) and suggests the closest known global.
* **Implicit Globals:** Warns when you forget the `local` keyword inside a function.
* **Unused Variables:** Granular detection for unused locals, functions, parameters, and loop variables.
* **Shadowing:** Warns when a local shadows an outer scope or global, providing a clickable link to the shadowed definition.
* **Unreachable Code:** Detects dead code after `return`, `break`, or `goto`.
* **Ambiguous Returns:** Catches Lua's infamous newline evaluation trap where expressions on the next line are accidentally returned.
* **Deprecation:** Warns when using symbols marked with `@deprecated`.

## Installation

### VS Code
Simply install the extension from the [VS Code Marketplace](https://marketplace.visualstudio.com/items?itemName=coalaura.lugo-vscode). The extension automatically detects your OS and architecture and runs the correct bundled Go binary. No external dependencies are required.

### Other Editors (Neovim, etc.)
Lugo is entirely editor-agnostic. You can download the standalone LSP binaries for Windows, Linux, and macOS from the [GitHub Releases](https://github.com/coalaura/lugo/releases) page and attach them to your client of choice using standard LSP configurations over `stdio`.

## Configuration

You can configure Lugo via your VS Code `settings.json` (also available via the settings UI under **Extensions -> Lugo LSP**):

**Workspace**
* `lugo.workspace.libraryPaths`: An array of absolute paths to external Lua libraries to index.
* `lugo.workspace.ignoreGlobs`: Additional glob patterns to ignore during indexing.

**Environment**
* `lugo.environment.knownGlobals`: Global variables to ignore when reporting undefined globals. Supports wildcards (e.g., `N_0x*`).

**Diagnostics**
* `lugo.diagnostics.undefinedGlobals`: Toggle undefined global warnings.
* `lugo.diagnostics.implicitGlobals`: Toggle warnings for forgetting the `local` keyword.
* `lugo.diagnostics.unusedLocal`: Toggle unused local variable detection.
* `lugo.diagnostics.unusedFunction`: Toggle unused local function detection.
* `lugo.diagnostics.unusedParameter`: Toggle unused parameter detection.
* `lugo.diagnostics.unusedLoopVar`: Toggle unused loop variable detection.
* `lugo.diagnostics.shadowing`: Toggle warnings when a local shadows an outer scope or global.
* `lugo.diagnostics.unreachableCode`: Toggle graying out unreachable code.
* `lugo.diagnostics.ambiguousReturns`: Toggle warnings for expressions accidentally returned due to newlines.
* `lugo.diagnostics.deprecated`: Toggle warnings for usage of `@deprecated` symbols.

**Editor**
* `lugo.inlayHints.parameterNames`: Enable inline parameter name hints for function and method calls.
* `lugo.features.documentHighlight`: Enable document highlights for variables and function/method calls.
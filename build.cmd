@echo off

if not exist vscode\bin (
	mkdir vscode\bin
)

go build -trimpath -o .\vscode\bin\lugo-win32-x64.exe
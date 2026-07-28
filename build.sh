mkdir -p vscode/bin

go build -trimpath -o ./vscode/bin/lugo-linux-x64

chmod +x ./vscode/bin/lugo-linux-x64
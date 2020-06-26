rm lib.wasm
GOARCH=wasm GOOS=js go build -o lib.wasm main.go
spark -port=8080
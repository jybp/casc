build: 
	@go build

run: 
	@cd ./cmd/example && go build && ./example

test:
	@go test -race -v ./...
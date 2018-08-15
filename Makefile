build: 
	@go build

run: 
	@cd ./cmd/casclist && go build && ./casclist -app d3 -region eu -cdn eu -v

test:
	@go test -race -v ./...
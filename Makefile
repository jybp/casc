build: 
	@cd ./d3/cmd/parser && go build

run: 
	@cd ./d3/cmd/parser && go build && ./parser

test:
	go test -v ./...
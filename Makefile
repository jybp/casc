build: 
	@go build

test:
	@go test -race -v ./...

online: 
	@cd ./cmd/casc-explorer && go build && ./casc-explorer -v -app d3 -region eu -cdn eu 

local: 
	@cd ./cmd/casc-explorer && go build && ./casc-explorer -v -dir "/Applications/Diablo III"

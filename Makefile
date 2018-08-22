build: 
	@go build

test:
	@go test -race -v ./...

online: 
	@cd ./cmd/casc-explorer && go build && ./casc-explorer -v -app w3 -region eu -cdn eu -cache cache

local: 
	@cd ./cmd/casc-explorer && go build && ./casc-explorer -v -dir "/Applications/Warcraft III"

app = d3
pattern = enUS/Data_D3/Locale/enUS/Cutscenes/*.sbt
dir = /Applications/Diablo III

.PHONY: build
build: 
	@go build

.PHONY: test
test:
	@go test -race -v ./...

.PHONY: online
online: 
	@cd ./cmd/casc-extract && go build && ./casc-extract -app $(app) -region eu -cdn eu -cache cache -pattern "$(pattern)" -o "online/$(app)" -v

.PHONY: local
local: 
	@cd ./cmd/casc-extract && go build && ./casc-extract -dir "$(dir)" -pattern "$(pattern)" -o "local/$(app)" -v

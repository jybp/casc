app = d3
pattern = aenUS/SoundBank/D_x1_fortress_SpiritWell_02_Wizard/*
dir = /Applications/Diablo III
output = extract/$(app)

.PHONY: build
build: 
	cd ./cmd/casc-extract && go build

.PHONY: test
test:
	@go test -race -v ./...

.PHONY: online
online: build
	@cd ./cmd/casc-extract && ./casc-extract -app $(app) -region eu -cdn eu -cache cache -pattern "$(pattern)" -o "$(output)/online" -v

.PHONY: local
local: build
	@cd ./cmd/casc-extract && ./casc-extract -dir "$(dir)" -pattern "$(pattern)" -o "$(output)/local" -v

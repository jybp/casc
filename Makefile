build: 
	@cd ./cmd; go build

run: 
	@cd ./cmd; go build; ./cmd

test:
	go test -v ./...
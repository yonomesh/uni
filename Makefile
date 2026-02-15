.PHONY: build clean test
build:
	- go build -o ./build/uni ./unicmd/uni
clean:
	- rm -rf cover.out cover.html
	- rm -rf ./build
test:
	- go test -coverprofile=cover.out ./...
	- go tool cover -html=cover.out -o cover.html
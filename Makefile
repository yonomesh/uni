.PHONY: test clean push
test:
	- go test -coverprofile=cover.out ./...
	- go tool cover -html=cover.out -o cover.html
clean:
	- rm -rf cover.out cover.html
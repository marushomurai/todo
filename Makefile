.PHONY: check build install

check:
	cd ~/Developer/Projects/todo && go vet ./...
	cd ~/Developer/Projects/todo && go test ./...

build:
	cd ~/Developer/Projects/todo && go build -o todo ./cmd/todo

install: check
	cd ~/Developer/Projects/todo && go install ./cmd/todo

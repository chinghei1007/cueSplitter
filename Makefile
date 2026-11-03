.PHONY: setup check-deps build frontend test e2e run clean

setup:
	./scripts/setup.sh

check-deps:
	./scripts/setup.sh --check

build:
	./scripts/build.sh

frontend:
	npm --prefix frontend ci
	npm --prefix frontend run build

test: frontend
	go test ./...

e2e:
	./scripts/e2e.sh

run: build
	./bin/cue-splitter

clean:
	go clean
	rm -rf bin

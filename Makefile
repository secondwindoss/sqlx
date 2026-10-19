.ONESHELL:
SHELL = /bin/sh
.SHELLFLAGS = -ec

BASE_PACKAGE := github.com/secondwindoss/sqlx

tooling:
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install golang.org/x/tools/cmd/goimports@latest

has-changes:
	git diff --exit-code --quiet HEAD --

lint:
	go vet ./...
	staticcheck -checks=all ./...
	cd dbtest && go vet ./... && staticcheck -checks=all ./...

fmt:
	goimports -local $(BASE_PACKAGE) -w .

vuln-check:
	govulncheck ./...
	cd dbtest && govulncheck ./...

test:
	go test -race -count=1 ./...

test-race: test
	cd dbtest && go test -v -race -count=1 ./...

update-dependencies:
	go mod tidy
	cd dbtest && go get -u -t -v ./... && go mod tidy

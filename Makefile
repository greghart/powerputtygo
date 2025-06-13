# Makefile to help standardize development for all our modules.

GOFMT_FILES = $(shell go list -f '{{.Dir}}' -m | grep -v '/pb')
GO_FILES = $(shell find . -name \*.go)
MD_FILES = $(shell find . -name \*.md)

# diff-check runs git-diff and fails if there are any changes.
diff-check:
	@FINDINGS="$$(git status -s -uall)" ; \
		if [ -n "$${FINDINGS}" ]; then \
			echo "Changed files:\n\n" ; \
			echo "$${FINDINGS}\n\n" ; \
			echo "Diffs:\n\n" ; \
			git diff ; \
			git diff --cached ; \
			exit 1 ; \
		fi
.PHONY: diff-check

# golangci-lint team does not recommend using go tool, so this is an implicit dependency.
lint:
	@echo ${GOFMT_FILES} | xargs golangci-lint run
.PHONY: lint

# TODO: Can use go test work in 1.25
test:
	@echo ${GOFMT_FILES} | xargs go test \
		-shuffle=on \
		-count=1 \
		-short \
		-timeout=5m
.PHONY: test

test-acc:
	@go test \
		-shuffle=on \
		-count=1 \
		-race \
		-timeout=10m \
		./... \
		-coverprofile=coverage.out
.PHONY: test-acc

test-coverage:
	@go tool cover -func=./coverage.out
.PHONY: test-coverage

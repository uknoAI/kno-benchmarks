# The commands CI runs, so that "it passed locally" means the same thing.
#
# There is deliberately no `bench` target and no `func Benchmark` anywhere in
# this repository: uknoAI/kno's `make bench-diff` is a tripwire on exactly that
# pattern, and this repository measures a shipped binary from the outside
# instead. See METHODOLOGY.md.

GO ?= go
BASE ?= origin/main

.PHONY: check
check: fmt-check vet test no-benchmarks lint generated append-only ## Everything CI runs on a PR
	@printf '\033[32m  OK  \033[0m all gates passed\n'

.PHONY: fmt-check
fmt-check: ## gofmt is clean
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then printf '\033[31m FAIL \033[0m gofmt would change:\n%s\n' "$$out"; exit 1; fi

.PHONY: vet
vet: ## go vet
	@$(GO) vet ./...

.PHONY: test
test: ## Unit tests. No network, no kno binary, no money.
	@$(GO) test -race -shuffle=on ./...

.PHONY: no-benchmarks
no-benchmarks: ## This repository must contain no Go benchmark
	@if grep -rqn '^func Benchmark' --include='*_test.go' .; then \
		printf '\033[31m FAIL \033[0m a Go benchmark exists here; it belongs in uknoAI/kno behind make bench-diff\n'; \
		exit 1; \
	fi
	@printf '\033[32m  OK  \033[0m no Go benchmark present\n'

.PHONY: lint
lint: ## Matrix and workflow rules
	@$(GO) run ./cmd/knobench lint

.PHONY: generated
generated: ## SUMMARY.md and results/latest.json are up to date
	@$(GO) run ./cmd/knobench summarize --check

.PHONY: summarize
summarize: ## Regenerate SUMMARY.md and results/latest.json
	@$(GO) run ./cmd/knobench summarize

.PHONY: append-only
append-only: ## results/ may only gain files
	@./scripts/append-only-check.sh $(BASE)

.PHONY: help
help: ## List targets
	@grep -hE '^[a-z-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

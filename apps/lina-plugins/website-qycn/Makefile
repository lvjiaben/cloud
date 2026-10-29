# Cross-platform commands contain no shell-specific environment assignments.
.PHONY: install build package test
install:
	pnpm install --frozen-lockfile
build package:
	pnpm run package
test:
	pnpm test

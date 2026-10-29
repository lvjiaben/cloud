.PHONY: install build test package dev
install:
	pnpm install --ignore-workspace --frozen-lockfile
build:
	pnpm run build
test:
	GOWORK=off go test ./... -count=1
	pnpm test
package: build
	pnpm run package
dev:
	pnpm run dev

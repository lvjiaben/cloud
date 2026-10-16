# Independent template build; all paths are relative to this plugin.
.PHONY: install build package test
install:
	pnpm install --frozen-lockfile
package:
	pnpm run package
test:
	GOWORK=off go test ./... -count=1
build:
	pnpm run package

# Independent template build; all paths are relative to this plugin.
.PHONY: install build package test
install:
	pnpm install --frozen-lockfile
package:
	pnpm run package
test:
	node scripts/test.mjs
build:
	pnpm run package

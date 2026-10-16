# Independent template build; no repository tools are required.
.PHONY: install build package test
install:
	pnpm install --frozen-lockfile
	pnpm --dir frontend/customer install --frozen-lockfile
build:
	pnpm run build
	pnpm run package
package:
	pnpm run package
test:
	pnpm test

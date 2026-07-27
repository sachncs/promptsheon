.PHONY: install dev build preview smoke typecheck

# Thin wrapper around the repo-root Makefile's `web-*` targets so you
# can `make dev` from this directory without a parent chdir.

install:
	cd .. && $(MAKE) web-install

dev:
	cd .. && $(MAKE) web-dev

build:
	cd .. && $(MAKE) web-build

preview:
	cd .. && $(MAKE) web-build && npx vite preview --port 4173

smoke:
	cd .. && $(MAKE) web-smoke

typecheck:
	node --check src/main.js && node --check src/api.js && node --check src/router.js

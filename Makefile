SHELL := /usr/bin/env bash

BUDGET_IMAGE_TAG ?= budget:dev
SQLC_VERSION := v1.31.1

.PHONY: help generate generate-sqlc generate-api generate-check fmt fmt-check test \
	test-integration web-install web-check api-check ios-check docker-build check release-check

help:
	@echo "Budget development targets"
	@echo "  make generate          Generate sqlc and OpenAPI bindings"
	@echo "  make fmt               Format Go code"
	@echo "  make test              Run Go tests that do not require Docker"
	@echo "  make test-integration  Run PostgreSQL integration tests"
	@echo "  make ios-check         Test the API package and build the iOS simulator app"
	@echo "  make check             Run local static checks and unit tests"
	@echo "  make release-check     Run complete release validation"

generate: web-install generate-sqlc generate-api

generate-sqlc:
	cd database && go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

generate-api:
	cd apps/server && go tool oapi-codegen -config ../../api/oapi-codegen.yaml ../../api/openapi.yaml
	cd apps/web && npm run api:generate
	cp api/openapi.yaml apps/ios/BudgetAPI/Sources/BudgetAPI/openapi.yaml

generate-check: generate
	git diff --exit-code -- \
		apps/ios/BudgetAPI/Sources/BudgetAPI/openapi.yaml \
		apps/server/internal/api/openapi \
		apps/server/internal/platform/postgres/sqlc \
		apps/web/src/api/generated

fmt:
	cd apps/server && gofmt -w $$(find . -name '*.go' -type f)

fmt-check:
	@files="$$(find apps/server -name '*.go' -type f)"; \
	if [[ -n "$$files" ]]; then \
		unformatted="$$(gofmt -l $$files)"; \
		[[ -z "$$unformatted" ]] || { echo "Unformatted Go files:"; echo "$$unformatted"; exit 1; }; \
	fi

test:
	cd apps/server && go test ./...

test-integration:
	cd apps/server && go test -tags=integration ./...

web-install:
	cd apps/web && npm ci

web-check: web-install
	cd apps/web && npm run typecheck
	cd apps/web && npm test -- --run

api-check: web-install
	cd apps/web && npm run api:lint

IOS_TEST_DESTINATION ?= platform=iOS Simulator,name=iPhone 17,OS=latest

ios-check:
	cd apps/ios/BudgetAPI && \
		DEVELOPER_DIR="$${DEVELOPER_DIR:-/Applications/Xcode.app/Contents/Developer}" \
		xcrun swift test --quiet
	DEVELOPER_DIR="$${DEVELOPER_DIR:-/Applications/Xcode.app/Contents/Developer}" \
		xcodebuild -project apps/ios/Budget.xcodeproj -scheme Budget \
		-configuration Debug -destination "$(IOS_TEST_DESTINATION)" \
		-derivedDataPath "$${TMPDIR:-/tmp}/budget-derived-data" \
		-skipPackagePluginValidation \
		CODE_SIGNING_ALLOWED=NO test -quiet

docker-build:
	docker build -f docker/Dockerfile -t "$(BUDGET_IMAGE_TAG)" .

check: fmt-check generate-check api-check test web-check

release-check: check test-integration web-check docker-build

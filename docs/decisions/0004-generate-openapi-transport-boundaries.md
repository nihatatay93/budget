# ADR 0004: Generate OpenAPI Transport Boundaries

## Status

Accepted

## Context

The Go server, React application, and SwiftUI application share one OpenAPI contract. Purely
handwritten request and response types can drift between those clients, while allowing
generated code throughout the domain or UI would couple business behavior to a generator.

The Go backend uses the standard library router. The web application already has a small
handwritten API wrapper, and Apple provides a build plugin designed for Swift OpenAPI client
generation.

## Decision

Treat `api/openapi.yaml` as the source of truth for HTTP operations and wire formats.

- Go uses pinned `oapi-codegen` tooling to generate models and strict `net/http` server
  interfaces under `apps/server/internal/api/openapi`. Domain packages do not import the
  generated package.
- TypeScript uses pinned `openapi-typescript` tooling to generate checked-in types under
  `apps/web/src/api/generated`. UI code accesses them through the handwritten `src/api`
  boundary rather than importing generated types throughout feature components.
- Swift uses Apple's Swift OpenAPI Generator build plugin with OpenAPIRuntime and the
  URLSession transport. Its generated code is build output and is not committed. Integration
  is added with the first native API workflow rather than introducing unused package
  dependencies to the initial app target. A local `apps/ios/BudgetAPI` package owns the
  plugin and transport dependencies. `make generate-api` synchronizes the authoritative
  contract into that package because the build plugin reads inputs from its target source
  directory; generated Swift remains uncommitted build output.

Go and TypeScript generated outputs are committed and checked for drift in CI. Generator
versions and configuration are reviewed like other build dependencies.

## Rationale

This catches contract drift while keeping domain logic, persistence, and presentation code
independent of generated representations. The Go choice preserves the accepted standard
library routing decision. The Swift choice follows the generator's intended build-time
workflow and avoids maintaining generated native sources manually.

## Consequences

- OpenAPI changes must regenerate Go and TypeScript outputs and synchronize the Swift plugin
  input in the same change.
- Transport handlers explicitly translate between generated types and domain types.
- Generated files are not edited by hand.
- iOS builds perform Swift generation once native API integration is present.
- Changing generators or allowing generated types across architectural boundaries requires
  revisiting this decision.

## Alternatives Considered

### Handwrite all transport types

This minimizes tooling but allows the three clients to diverge from the shared contract.

### Generate types directly into domain and feature packages

This reduces mapping code but makes business behavior depend on generator output and API
wire-format decisions.

### Commit Swift-generated source

Ahead-of-time Swift generation is supported, but the build plugin keeps output synchronized
without adding generated-source churn to the repository.

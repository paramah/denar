// Package api holds the types and the server interface generated from
// api/openapi.yaml. gen.go is generated; do not edit it by hand.
//
// Generator: oapi-codegen v2.8.0, pinned in the go:generate line below and
// run with "go run pkg@version", so it is not a dependency in go.mod. Its
// options are in oapi-codegen.yaml: std-http-server (net/http ServeMux),
// strict-server and nullable types (OpenAPI 3.1 nulls become
// nullable.Nullable[T]).
//
//	go generate ./internal/adapter/rest/api/...
//
// TestGeneratedCodeIsCurrent fails when gen.go is stale (skipped with
// -short, since it runs the generator). The generator does not check
// constraints such as minimum or enum on input; handlers and use cases do.
// Mount the handler with BaseURL "/api".
package api

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config oapi-codegen.yaml ../../../../api/openapi.yaml

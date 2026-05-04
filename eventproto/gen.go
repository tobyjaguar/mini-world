// gen.go declares the typegen generation directive. Running
// `go generate ./eventproto/...` (or `go generate ./...` from the worldsim
// repo root) regenerates `crossworlds/src/lib/eventproto.generated.ts`.
//
// `go generate` runs the directive with cwd = the directory of THIS file
// (eventproto/), so relative paths resolve from there.

package eventproto

//go:generate go run ../cmd/typegen ../../crossworlds/src/lib/eventproto.generated.ts

// Command capture-release instruments an isolated released source checkout.
// It is used only by the hosted migration bootstrap, never by the application.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

type target struct {
	file, name, operation, input, result string
}

func main() {
	root := flag.String("root", "", "isolated released repository")
	flag.Parse()
	if *root == "" {
		panic("--root is required")
	}
	pathInput := `map[string]any{"scope":in.Scope,"config":in.Config,"ingress_interface":in.IngressInterface,"flow":in.Flow}`
	returnInput := `map[string]any{"scope":in.Scope,"config":in.Config,"ingress_interface":in.IngressInterface,"flow":in.Flow,"forward":forward}`
	networkInput := `releaseNetworkInput(in)`
	targets := []target{
		{"firewall/asa.go", "ParseASAConfig", "parse_asa", `map[string]any{"config_base64":input}`, "releaseResult0"},
		{"firewall/panos.go", "ParsePANOSConfig", "parse_panos", `map[string]any{"config_base64":input}`, "releaseResult0"},
		{"firewall/fortigate.go", "ParseFortiGateConfig", "parse_fortigate", `map[string]any{"config_base64":input}`, "releaseResult0"},
		{"policy/evaluate.go", "ValidateScope", "validate_scope", `map[string]any{"scope":s}`, `map[string]any{"valid":true}`},
		{"policy/evaluate.go", "Validate", "validate_policy", `map[string]any{"bundle":b,"flow":f}`, `map[string]any{"valid":true}`},
		{"policy/evaluate.go", "Evaluate", "evaluate_policy", `map[string]any{"bundle":b,"flow":f}`, "releaseResult0"},
		{"pathanalysis/evaluate.go", "Evaluate", "evaluate_path", pathInput, "releaseResult0"},
		{"pathanalysis/evaluate.go", "EvaluateReturn", "evaluate_return", returnInput, "releaseResult0"},
		{"pathanalysis/exploration.go", "Explore", "explore_path", pathInput, `map[string]any{"candidates":releaseResult0,"truncated":releaseResult1}`},
		{"pathanalysis/exploration.go", "ExploreReturn", "explore_return", returnInput, `map[string]any{"candidates":releaseResult0,"truncated":releaseResult1}`},
		{"networkpath/evaluate.go", "Evaluate", "evaluate_network", networkInput, "releaseResult0"},
		{"networkpath/exploration.go", "Explore", "explore_network", networkInput, "releaseResult0"},
	}
	packages := map[string]bool{}
	for _, t := range targets {
		filename := filepath.Join(*root, "platform", filepath.FromSlash(t.file))
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
		must(err)
		found := false
		for _, declaration := range f.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name.Name != t.name {
				continue
			}
			found = true
			for i, field := range fn.Type.Results.List {
				if len(field.Names) != 0 {
					panic("capture requires unnamed release result: " + t.file + ":" + t.name)
				}
				field.Names = []*ast.Ident{ast.NewIdent(fmt.Sprintf("releaseResult%d", i))}
			}
			errorName := fmt.Sprintf("releaseResult%d", len(fn.Type.Results.List)-1)
			statement := fmt.Sprintf("defer func(){ releaseRecord(%q, %s, %s, %s) }()", t.operation, t.input, t.result, errorName)
			wrapper, err := parser.ParseFile(token.NewFileSet(), "defer.go", "package p; func f(){"+statement+"}", 0)
			must(err)
			stmt := wrapper.Decls[0].(*ast.FuncDecl).Body.List[0]
			fn.Body.List = append([]ast.Stmt{stmt}, fn.Body.List...)
		}
		if !found {
			panic("release function absent: " + t.file + ":" + t.name)
		}
		var out bytes.Buffer
		must(format.Node(&out, fset, f))
		must(os.WriteFile(filename, out.Bytes(), 0600))
		packages[filepath.Dir(t.file)] = true
	}
	for pkg := range packages {
		text := fmt.Sprintf(captureHelper, filepath.Base(pkg), filepath.Base(pkg))
		if pkg == "networkpath" {
			text += networkHelper
		}
		must(os.WriteFile(filepath.Join(*root, "platform", pkg, "release_capture.go"), []byte(text), 0600))
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

const captureHelper = `package %s

import (
 "crypto/sha256"
 "encoding/json"
 "os"
 "path/filepath"
 "strings"
 "sync"
)

var releaseCaptureMu sync.Mutex
var releaseCaptureSeen = map[[32]byte]bool{}

func releaseRecord(operation string, input, result any, err error) {
 // Worker transport failures in deliberately broken-worker tests are not domain results.
 if err != nil && strings.Contains(err.Error(), "analysis engine") { return }
 record := map[string]any{"operation":operation,"input":input}
 if err != nil { record["error"] = err.Error() } else { record["result"] = result }
 data, e := json.Marshal(record)
 if e != nil { panic(e) }
 releaseCaptureMu.Lock()
 defer releaseCaptureMu.Unlock()
 digest := sha256.Sum256(data)
 if releaseCaptureSeen[digest] { return }
 releaseCaptureSeen[digest] = true
 output := os.Getenv("RELEASE_CAPTURE_DIR")
 if output == "" { panic("RELEASE_CAPTURE_DIR is required") }
 f, e := os.OpenFile(filepath.Join(output, "%s.ndjson"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
 if e != nil { panic(e) }
 defer f.Close()
 if _, e = f.Write(append(data, '\n')); e != nil { panic(e) }
}
`

const networkHelper = `
func releaseNetworkInput(in Input) any {
 nodes := make([]any,0,len(in.Nodes))
 for _, n := range in.Nodes { nodes=append(nodes,map[string]any{"id":n.ID,"scope":n.Scope,"config":n.Config}) }
 return map[string]any{"nodes":nodes,"links":in.Links,"source":in.Source,"destination":in.Destination,"flow":in.Flow}
}
`

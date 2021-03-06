// client-gen-go tool generates code for the simlifed usage of the client package
//
// search in <path> for interface types with <suffix>
// and generate code for <package> with matching interface types
// and computed type names in <out> file
//
//
// Example:
// package		package to generate code for (default: main)
// path			path to search for interface types (default: .)
// out			file name for the generated code (default: client.http.go)
// suffix		suffix of the interface type name we are looking for (default: Service)
// goimports	path to the goimports tool (default: goimports)
//
// For a interface type named NodeService the following names will be computed:
//	- NodeImpl	type implementing NodeService
//				NodeImpl must exist
//  - Node		field name in Client type
//	- node		for initialization purpose only

package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"
)

const codeTemplate = `
// Code generated by client-gen-go; DO NOT EDIT.
// This file was generated by robots at
// {{ .Timestamp }}

package {{.Package}}

import (
	"net/http"
	"net/url"

)

// Client is a generated wrapper for a http client and detected services.
type Client struct {
	*httpclient.Client

	// Services used for communicating with the API
{{- range .Services }}
	{{ printf "%s %s" .FieldName .InterfaceName }}
{{- end }}
}

// NewClient returns a new API client.
func NewClient(baseURL string, opts ...httpclient.Opt) (*Client, error) {

	client, err := httpclient.New(baseURL, opts...)
	if err != nil {
		return nil, err
	}

	// services
{{- range .Services }}
{{ printf "%s := &%s{client: client}" .VarName .TypeName }}
{{- end }}

	return &Client{
		client,
{{- range .Services }}
{{ printf "%s," .VarName }}
{{- end }}
	}, nil
}
`

// service contains all names for the code generation
type service struct {
	FieldName     string
	VarName       string
	TypeName      string
	InterfaceName string
}

// nolint: gochecknoglobals
var (
	targetPackage string
	sourcePath    string
	outputFile    string
	svcSuffix     string
	goImports     string
	force         bool
)

// nolint: gochecknoinits
func init() {
	flag.StringVar(&targetPackage, "package", "main", "package name for the generated code")
	flag.StringVar(&sourcePath, "path", ".", "path to scan for services")
	flag.StringVar(&outputFile, "out", "httpclient.go", "output filename")
	flag.StringVar(&svcSuffix, "suffix", "Service", "service suffix")
	flag.StringVar(&goImports, "goimports", "goimports", "path to goimports tool")
	flag.BoolVar(&force, "force", false, "write file even it already exists")
}

// nolint: gocognit, gocyclo
func main() {
	flag.Parse()

	// check if file exists
	if _, err := os.Stat(outputFile); err == nil && !force {
		log.Fatalf("%s already exists - remove or choose a different file name\n", outputFile)
	}

	// get all services
	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, sourcePath, nil, parser.AllErrors)
	if err != nil {
		log.Fatal(err)
	}

	services := []service{}

	for _, p := range pkgs {
		for _, f := range p.Files {
			for _, d := range f.Decls {
				if t, ok := d.(*ast.GenDecl); ok {
					if t.Tok != token.TYPE {
						continue
					}

					for _, s := range t.Specs {
						if ts, ok := s.(*ast.TypeSpec); ok {
							if _, ok := ts.Type.(*ast.InterfaceType); ok {
								if !strings.HasSuffix(ts.Name.String(), svcSuffix) {
									continue
								}

								name := strings.TrimSuffix(ts.Name.String(), svcSuffix)
								typeName := fmt.Sprintf("%s.%sImpl", p.Name, name)              // {name}Impl
								interfaceName := fmt.Sprintf("%s.%s", p.Name, ts.Name.String()) // {name}Service

								if p.Name == targetPackage {
									typeName = fmt.Sprintf("%sImpl", name)
									interfaceName = ts.Name.String()
								}

								services = append(services, service{
									FieldName:     name,
									VarName:       strings.ToLower(name),
									TypeName:      typeName,
									InterfaceName: interfaceName,
								})
							}
						}
					}
				}
			}
		}
	}

	// sort :-)
	sort.Slice(services, func(i, j int) bool {
		return services[i].InterfaceName < services[j].InterfaceName
	})

	// render template
	t := template.Must(template.New("Client Type Template").Parse(codeTemplate))

	f, err := os.Create(outputFile)
	if err != nil {
		log.Fatal(errors.Wrapf(err, "could not create output file %s", outputFile))
	}

	_ = t.Execute(f, struct {
		Timestamp time.Time
		Path      string
		Package   string
		Services  []service
	}{
		Timestamp: time.Now(),
		Path:      sourcePath,
		Package:   targetPackage,
		Services:  services,
	})

	_ = f.Close()

	// format code and fix imports
	// nolint: gosec // G204: Subprocess launched with variable
	if out, err := exec.Command(goImports, "-w", "-l", outputFile).CombinedOutput(); err != nil {
		log.Fatal(errors.Wrap(err, string(out)))
	}

	fmt.Printf("%s generated\n", outputFile)
}

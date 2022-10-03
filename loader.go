package hclconfig

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/mattn/go-isatty"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"golang.org/x/term"
)

var defaultLoader *Loader = New()

// Loader represents config loader.
type Loader struct {
	diagsWriter hcl.DiagnosticWriter
	diagsOutput io.Writer
	width       uint
	color       bool

	variables map[string]cty.Value
	functions map[string]function.Function
}

// New creates a Loader instance.
func New() *Loader {
	width, _, err := term.GetSize(0)
	if err != nil || width <= 0 {
		width = 400
	}
	l := &Loader{
		diagsOutput: os.Stderr,
		width:       uint(width),
		color:       isatty.IsTerminal(os.Stdout.Fd()),
	}
	l.Functions(defaultFunctions)
	return l
}

// NewEvalContext creates a new evaluation context.
func NewEvalContext() *hcl.EvalContext {
	return defaultLoader.NewEvalContext()
}

// NewEvalContext creates a new evaluation context.
func (l *Loader) NewEvalContext() *hcl.EvalContext {
	return &hcl.EvalContext{
		Variables: l.variables,
		Functions: l.functions,
	}
}

// DiagnosticWriter sets up a Writer to write the diagnostic when an error occurs in the Loader.
func DiagnosticWriter(w hcl.DiagnosticWriter) {
	defaultLoader.DiagnosticWriter(w)
}

// DiagnosticWriter sets up a Writer to write the diagnostic when an error occurs in the Loader.
func (l *Loader) DiagnosticWriter(w hcl.DiagnosticWriter) {
	l.diagsWriter = w
}

// DefaultDiagnosticOutput specifies the standard diagnostic output destination. If a separate DiagnosticWriter is specified, that setting takes precedence.
func DefaultDiagnosticOutput(w io.Writer) {
	defaultLoader.DefaultDiagnosticOutput(w)
}

// DefaultDiagnosticOutput specifies the standard diagnostic output destination. If a separate DiagnosticWriter is specified, that setting takes precedence.
func (l *Loader) DefaultDiagnosticOutput(w io.Writer) {
	l.diagsOutput = w
}

// Functions adds functions used during HCL decoding.
func Functions(functions map[string]function.Function) {
	defaultLoader.Functions(functions)
}

// Functions adds functions used during HCL decoding.
func (l *Loader) Functions(functions map[string]function.Function) {
	l.functions = mergeFunctions(l.functions, functions)
}

// Variables adds variables used during HCL decoding.
func Variables(variables map[string]cty.Value) {
	defaultLoader.Variables(variables)
}

// Variables adds variables used during HCL decoding.
func (l *Loader) Variables(variables map[string]cty.Value) {
	l.variables = mergeVariables(l.variables, variables)
}

// BodyDecoder is an interface for custom decoding methods.
// If the load target does not satisfy this interface, gohcl.DecodeBody is used, but if it does, the functions of this interface are used.
type BodyDecoder interface {
	DecodeBody(hcl.Body, *hcl.EvalContext) hcl.Diagnostics
}

// LoadWithBody assigns a value to `val` using a parsed hcl.Body and hcl.EvalContext.
// mainly used to achieve partial loading when implementing Restrict functions.
func LoadWithBody(cfg interface{}, body hcl.Body) hcl.Diagnostics {
	return defaultLoader.LoadWithBody(cfg, body)
}

// LoadWithBody assigns a value to `val` using a parsed hcl.Body and hcl.EvalContext.
// mainly used to achieve partial loading when implementing Restrict functions.
func (l *Loader) LoadWithBody(cfg interface{}, body hcl.Body) hcl.Diagnostics {
	ctx := l.NewEvalContext()
	remain, locals, diags := localVariables(body, ctx)
	if diags.HasErrors() {
		return diags
	}
	ctx = mergeEvalContextVariables(ctx, locals)
	variables, variablesDiags := impliedVariables(remain, ctx, cfg)
	diags = append(diags, variablesDiags...)
	if diags.HasErrors() {
		return diags
	}
	ctx = mergeEvalContextVariables(ctx, variables)
	diags = append(diags, DecodeBody(remain, ctx, cfg)...)
	return diags
}

func DecodeBody(body hcl.Body, ctx *hcl.EvalContext, cfg interface{}) hcl.Diagnostics {
	if decoder, ok := cfg.(BodyDecoder); ok {
		return decoder.DecodeBody(body, ctx)
	}
	diags := gohcl.DecodeBody(body, ctx, cfg)
	if diags.HasErrors() {
		return diags
	}

	restrictDiags := restrict(body, ctx, cfg)
	diags = append(diags, restrictDiags...)
	return diags
}

func (l *Loader) writeDiags(diags hcl.Diagnostics, files map[string]*hcl.File) error {
	if diags.HasErrors() {
		w := l.diagsWriter
		if w == nil {
			w = hcl.NewDiagnosticTextWriter(l.diagsOutput, files, l.width, l.color)
		}
		w.WriteDiagnostics(diags)
		return fmt.Errorf("%d errors occurred. See diagnostics for details", len(diags.Errs()))
	}
	return nil
}

// Load considers `paths` as a configuration file county written in HCL and reads *.hcl and *.hcl.json.
// and assigns the decoded values to the `cfg` values.
func Load(cfg interface{}, paths ...string) error {
	return defaultLoader.Load(cfg, paths...)
}

// Load considers `paths` as a configuration file county written in HCL and reads *.hcl and *.hcl.json.
// and assigns the decoded values to the `cfg` values.
func (l *Loader) Load(cfg interface{}, paths ...string) error {
	parser := hclparse.NewParser()
	var diags hcl.Diagnostics
	for _, path := range paths {
		diags = append(diags, l.parse(parser, path)...)
	}
	files := parser.Files()
	if diags.HasErrors() {
		return l.writeDiags(diags, files)
	}
	parsed := make([]*hcl.File, 0, len(files))
	for _, f := range files {
		parsed = append(parsed, f)
	}
	body := hcl.MergeFiles(parsed)
	diags = append(diags, l.LoadWithBody(cfg, body)...)
	return l.writeDiags(diags, files)
}

func (l *Loader) parse(parser *hclparse.Parser, path string) hcl.Diagnostics {
	var diags hcl.Diagnostics
	if _, err := os.Stat(path); err != nil {
		diags = append(diags, NewDiagnosticError("path not found", err.Error(), nil))
		return diags
	}
	files, err := filepath.Glob(filepath.Join(path, "*.hcl"))
	if err != nil {
		diags = append(diags, NewDiagnosticError("list *.hcl failed", err.Error(), nil))
		return diags
	}
	for _, file := range files {
		_, parseDiags := parser.ParseHCLFile(file)
		diags = append(diags, parseDiags...)
	}
	files, err = filepath.Glob(filepath.Join(path, "*.hcl.json"))
	if err != nil {
		diags = append(diags, NewDiagnosticError("list *.hcl.json failed", err.Error(), nil))
		return diags
	}
	for _, file := range files {
		_, parseDiags := parser.ParseJSONFile(file)
		diags = append(diags, parseDiags...)
	}
	return diags
}

func LoadWithBytes(cfg interface{}, filename string, src []byte) error {
	return defaultLoader.LoadWithBytes(cfg, filename, src)
}

func (l *Loader) LoadWithBytes(cfg interface{}, filename string, src []byte) error {
	parser := hclparse.NewParser()
	var diags hcl.Diagnostics
	var file *hcl.File
	switch filepath.Ext(filename) {
	case ".hcl":
		file, diags = parser.ParseHCL(src, filename)
	case ".json":
		file, diags = parser.ParseJSON(src, filename)
	default:
		diags = append(diags, NewDiagnosticError("invalid file format", "ext suffix must .json or .hcl", nil))
	}
	if diags.HasErrors() {
		return l.writeDiags(diags, parser.Files())
	}
	diags = append(diags, l.LoadWithBody(cfg, file.Body)...)
	return l.writeDiags(diags, parser.Files())
}

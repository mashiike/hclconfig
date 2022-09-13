package hclconfig

import (
	"github.com/hashicorp/hcl/v2"
)

// NewDiagnosticError generates a new diagnostic error.
func NewDiagnosticError(summary string, detail string, r *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  summary,
		Detail:   detail,
		Subject:  r,
	}
}

// NewDiagnosticError generates a new diagnostic warn.
func NewDiagnosticWarn(summary string, detail string, r *hcl.Range) *hcl.Diagnostic {
	return &hcl.Diagnostic{
		Severity: hcl.DiagWarning,
		Summary:  summary,
		Detail:   detail,
		Subject:  r,
	}
}

// DiagnosticWriterFunc is an alias for a function that specifies how to output diagnostics.
type DiagnosticWriterFunc func(diag *hcl.Diagnostic) error

func (f DiagnosticWriterFunc) WriteDiagnostic(diag *hcl.Diagnostic) error {
	if f == nil {
		return nil
	}
	return f(diag)
}

func (f DiagnosticWriterFunc) WriteDiagnostics(diags hcl.Diagnostics) error {
	for _, diag := range diags {
		err := f.WriteDiagnostic(diag)
		if err != nil {
			return err
		}
	}
	return nil
}

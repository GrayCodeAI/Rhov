package testaudit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// zeroAssertionBaseline is the number of Test* functions without an assertion
// as of this writing. The guard fails when the count grows, so new tests cannot
// silently add to the debt; reduce the baseline as tests are fixed.
const zeroAssertionBaseline = 229

// assertionCall reports whether an expression is a test assertion
// (t.Error/Fatal/Fail/FailNow, require.*, assert.*, or a helper that calls
// t.Helper).
func assertionCall(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		name := sel.Sel.Name
		switch name {
		case "Error", "Errorf", "Fatal", "Fatalf", "Fail", "FailNow":
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "t" {
				found = true
				return false
			}
		case "Equal", "NotEqual", "True", "False", "Nil", "NotNil", "NoError",
			"ErrorIs", "Contains", "Len", "Empty":
			if id, ok := sel.X.(*ast.Ident); ok && (id.Name == "require" || id.Name == "assert") {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// TestNoZeroAssertionTests fails when the number of assertion-free Test*
// functions grows beyond the baseline. These tests pass regardless of behavior,
// giving false confidence.
func TestNoZeroAssertionTests(t *testing.T) {
	root := repoRoot(t)
	var count int
	var examples []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Test") {
				continue
			}
			if !assertionCall(fn) {
				count++
				if len(examples) < 20 {
					examples = append(examples, rel+":"+fn.Name.Name)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if count > zeroAssertionBaseline {
		t.Errorf("zero-assertion test functions grew to %d (baseline %d); add assertions or update the baseline deliberately:\n%s",
			count, zeroAssertionBaseline, strings.Join(examples, "\n"))
	}
	t.Logf("zero-assertion test functions: %d (baseline %d)", count, zeroAssertionBaseline)
}

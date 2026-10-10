// Package conformance holds checks that read the source rather than run it.
//
// ⚠ They parse Go into an AST, so comments (including the ones that name the
// forbidden calls in order to forbid them) are never matched (CLAUDE.md § 5).
package conformance

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// relPos prints a position relative to the repository, so CI logs carry no
// absolute local path (.claude/rules/git.md).
func relPos(t *testing.T, fset *token.FileSet, pos token.Pos) string {
	p := fset.Position(pos)
	rel, _ := filepath.Rel(repoRoot(t), p.Filename)
	return rel + ":" + strconv.Itoa(p.Line)
}

// productFiles are the non-test Go files that ship in the binary.
func productFiles(t *testing.T) []string {
	root := repoRoot(t)
	var out []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			// Test-only code: the fake DNS server and the final gate.
			if strings.HasPrefix(d.Name(), ".") || rel == "e2e" || rel == filepath.Join("internal", "dnstest") || rel == filepath.Join("internal", "tlstest") || rel == filepath.Join("internal", "conformance") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			out = append(out, p)
		}
		return nil
	})
	if len(out) == 0 {
		t.Fatal("no product files found")
	}
	return out
}

// forbidden are selectors that resolve or dial on their own, or switch off
// verification (.claude/rules/security.md § 1).
var forbidden = map[string]bool{
	"http.Get": true, "http.Head": true, "http.Post": true, "http.PostForm": true,
	"http.DefaultClient": true, "http.DefaultTransport": true,
	"net.Dial": true, "net.DialTimeout": true, "tls.Dial": true,
}

func TestNoSelfDialingCalls(t *testing.T) {
	fset := token.NewFileSet()
	for _, f := range productFiles(t) {
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && forbidden[id.Name+"."+x.Sel.Name] {
					t.Errorf("%s: %s.%s is forbidden in product code", relPos(t, fset, x.Pos()), id.Name, x.Sel.Name)
				}
			case *ast.KeyValueExpr:
				if id, ok := x.Key.(*ast.Ident); ok && id.Name == "InsecureSkipVerify" {
					t.Errorf("%s: InsecureSkipVerify is forbidden in product code", relPos(t, fset, x.Pos()))
				}
			}
			return true
		})
	}
}

// dialerAllowed are the only product files that may build a net.Dialer.
// ⚠ Every other dial must go through internal/dial (.claude/rules/security.md § 1).
var dialerAllowed = map[string]string{
	filepath.Join("internal", "dial", "dial.go"):      "the one dialer, with the policy hook",
	filepath.Join("cmd", "connect-doctor", "main.go"): "the operator's resolver address, never a target",
}

func TestOnlyInternalDialBuildsADialer(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	for _, f := range productFiles(t) {
		rel, _ := filepath.Rel(root, f)
		if _, ok := dialerAllowed[rel]; ok {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if x, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := x.X.(*ast.Ident); ok && id.Name == "net" && (x.Sel.Name == "Dialer" || x.Sel.Name == "DialTCP" || x.Sel.Name == "DialUDP" || x.Sel.Name == "DialIP") {
					p := fset.Position(x.Pos())
					t.Errorf("%s:%d: net.%s outside internal/dial", rel, p.Line, x.Sel.Name)
				}
			}
			return true
		})
	}
}

func TestGoModHasNoRequire(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "require") {
			t.Errorf("go.mod has %q: third-party modules need an ADR first (docs/adr/0006)", line)
		}
	}
}

/**
 * @file features.go
 * @brief Feature analyzer for Go code.
 */
package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

/**
 * @brief Holds identifier information.
 * @var name variable name
 * @var pos file position
 */
type info_t struct {
	name string
	pos  string
}

var allocs []string
var gostmt []string
var deferstmt []string
var appendstmt []string
var closures []string
var interfaces []string
var typeasserts []string
var multiret []string
var finalizers []string
var maps []info_t
var slices []info_t
var channels []info_t
var stringuse []info_t
var nmaptypes int
var imports map[string][]string
var lcount int

var verbose = false

/**
 * @brief Categorizes AST node types.
 * @param node expression to examine
 * @param name identifier associated with node
 * @param pos position string of node
 * @global maps tracked map declarations
 * @global slices tracked slice declarations
 * @global channels tracked channel declarations
 * @global stringuse tracked string uses
 */
func dotype(node ast.Expr, name string, pos string) {
	switch x := node.(type) {
	case *ast.MapType:
		i := info_t{name, pos}
		maps = append(maps, i)
	case *ast.ArrayType:
		i := info_t{name, pos}
		slices = append(slices, i)
	case *ast.ChanType:
		i := info_t{name, pos}
		channels = append(channels, i)
	case *ast.Ident:
		if x.Name == "string" {
			i := info_t{name, pos}
			stringuse = append(stringuse, i)
		}
	}
}

/**
 * @brief Returns the first identifier name if present.
 * @param names slice of identifiers
 * @return string name or empty when none
 */
func doname(names []*ast.Ident) string {
	if len(names) > 0 {
		return names[0].String()
	}
	return ""
}

/**
 * @brief Determines if the expression list begins with an append call.
 * @param exprs list of expressions
 * @return true when append is called
 */
func is_append_call(exprs []ast.Expr) bool {
	if len(exprs) == 0 {
		return false
	}
	if call, ok := exprs[0].(*ast.CallExpr); ok {
		if fun, ok := call.Fun.(*ast.Ident); ok {
			return fun.Name == "append"
		}
	}
	return false
}

/**
 * @brief Detects a make call as the first expression.
 * @param exprs list of expressions
 * @return true when make is invoked
 */
func is_make_call(exprs []ast.Expr) bool {
	if len(exprs) == 0 {
		return false
	}
	if call, ok := exprs[0].(*ast.CallExpr); ok {
		if fun, ok := call.Fun.(*ast.Ident); ok {
			return fun.Name == "make"
		}
	}
	return false
}

/**
 * @brief Checks for a new call in the expression list.
 * @param exprs list of expressions
 * @return true when new is the first call
 */
func is_new_call(exprs []ast.Expr) bool {
	if len(exprs) == 0 {
		return false
	}
	if call, ok := exprs[0].(*ast.CallExpr); ok {
		if fun, ok := call.Fun.(*ast.Ident); ok {
			return fun.Name == "new"
		}
	}
	return false
}

/**
 * @brief Checks if expressions allocate memory via composite literals.
 * @param exprs list of expressions
 * @return true when allocation detected
 */
func is_alloc_call(exprs []ast.Expr) bool {
	if len(exprs) == 0 {
		return false
	}
	if u, ok := exprs[0].(*ast.UnaryExpr); ok && u.Op == token.AND {
		if _, ok := u.X.(*ast.CompositeLit); ok {
			return true
		}
	}
	return false
}

/**
 * @brief Determines if a call expression invokes runtime.SetFinalizer.
 * @param c call expression
 * @return true when SetFinalizer is used
 */
func is_set_finalizer(c *ast.CallExpr) bool {
	if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
		return sel.Sel.Name == "SetFinalizer"
	}
	return false
}

/**
 * @brief Walks the AST node collecting language features.
 * @param node current AST node
 * @param fset token file set for position info
 * @return always true to continue traversal
 * @global various slices tracking feature usage
 */
func donode(node ast.Node, fset *token.FileSet) bool {
	// ast.Print(fset,node)
	switch x := node.(type) {
	case *ast.Field:
		pos := fset.Position(node.Pos()).String()
		dotype(x.Type, doname(x.Names), pos)
	case *ast.MapType:
		// pos := fset.Position(node.Pos()).String()
		nmaptypes++
	case *ast.GenDecl:
		pos := fset.Position(node.Pos()).String()
		for _, spec := range x.Specs {
			switch y := spec.(type) {
			case *ast.ValueSpec:
				name := doname(y.Names)
				for _, val := range y.Values {
					switch z := val.(type) {
					case *ast.CompositeLit:
						dotype(z.Type, name, pos)
					}
				}
			}
		}
	case *ast.GoStmt:
		gostmt = append(gostmt, fset.Position(node.Pos()).String())
	case *ast.DeferStmt:
		deferstmt = append(deferstmt, fset.Position(node.Pos()).String())
	case *ast.AssignStmt:
		pos := fset.Position(node.Pos()).String()
		if is_append_call(x.Rhs) {
			appendstmt = append(appendstmt, pos)
		}
		if is_make_call(x.Rhs) {
			allocs = append(allocs, pos)
		}
		if is_new_call(x.Rhs) {
			allocs = append(allocs, pos)
		}
		if is_alloc_call(x.Rhs) {
			// ast.Print(fset, x)
			// fmt.Printf("pos %s\n", pos)
			allocs = append(allocs, pos)
		}

	case *ast.FuncLit:
		pos := fset.Position(node.Pos()).String()
		closures = append(closures, pos)
	case *ast.InterfaceType:
		pos := fset.Position(node.Pos()).String()
		interfaces = append(interfaces, pos)
	case *ast.TypeAssertExpr:
		pos := fset.Position(node.Pos()).String()
		typeasserts = append(typeasserts, pos)
	case *ast.FuncDecl:
		pos := fset.Position(node.Pos()).String()
		// ast.Print(fset, x)
		t := x.Type
		if t.Results != nil && len(t.Results.List) > 1 {
			multiret = append(multiret, pos)
		}
	case *ast.ExprStmt:
		pos := fset.Position(node.Pos()).String()
		switch y := x.X.(type) {
		case *ast.CallExpr:
			// ast.Print(fset, x)
			if is_set_finalizer(y) {
				finalizers = append(finalizers, pos)
			}
		}
	}
	return true
}

/**
 * @brief Adds a file to the import usage map.
 * @param f file name
 * @param imp import path
 * @global imports map of import to files
 */
func addimport(f string, imp string) {
	s, ok := imports[imp]
	if ok {
		imports[imp] = append(s, f)
	} else {
		imports[imp] = []string{f}
	}
}

/**
 * @brief Counts lines in a reader using bufio.Scanner.
 * @param r input reader
 * @return number of lines and an error if any
 */
func lineCounter(r io.Reader) (int, error) {
	scanner := bufio.NewScanner(r)
	count := 0
	for scanner.Scan() {
		count++
	}
	return count, scanner.Err()
}

/**
 * @brief Processes a single Go source file.
 * @param path file path to parse
 * @global lcount running line count
 */
func dofile(path string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, s := range f.Imports {
		addimport(fset.Position(f.Package).String(), s.Path.Value)
	}
	ast.Inspect(f, func(node ast.Node) bool {
		return donode(node, fset)
	})

	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	l, err := lineCounter(file)
	if err != nil {
		log.Fatal(err)
	}
	lcount += l
}

/**
 * @brief Returns thousandths ratio of x over line count.
 * @param x value to scale
 * @return scaled result
 */
func frac(x int) float64 {
	return (float64(x) / float64(lcount)) * 1000
}

/**
 * @brief Prints feature information for info_t slices.
 * @param n label name
 * @param x slice of info_t
 */
func printi(n string, x []info_t) {
	fmt.Printf("%s & %.2f \\ \n", n, frac(len(x)))
	if verbose {
		for _, i := range x {
			fmt.Printf("\t%s (%s)\n", i.name, i.pos)
		}
	}
}

/**
 * @brief Prints feature summary for string slices.
 * @param n label name
 * @param x slice of strings
 */
func print(n string, x []string) {
	fmt.Printf("%s & %.2f \\ \n", n, frac(len(x)))
	if verbose {
		for _, i := range x {
			fmt.Printf("\t%s\n", i)
		}
	}
}

/**
 * @brief Prints map-based feature statistics.
 * @param n label name
 * @param m map from string to file list
 */
func printm(n string, m map[string][]string) {
	fmt.Printf("%s & %.2f \\ \n", n, frac(len(m)))
	if verbose {
		for k, v := range m {
			fmt.Printf("\t%s (%d): %v\n", k, len(v), v)
		}
	}
}

/**
 * @brief Entry point for feature analysis tool.
 * @global lcount running line total
 */
func main() {
	if len(os.Args) != 2 {
		fmt.Println("features.go <path>")
		return
	}
	imports = make(map[string][]string)
	dir := os.Args[1]
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(strings.TrimSpace(path)) == ".go" {
			dofile(path)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("error %v\n", err)

	}

	fmt.Printf("Line count %d\n", lcount)

	// printi("Maps", maps)
	// printi("Slices", slices)
	// printi("Channels", channels)
	// printi("Strings", stringuse)
	// print("Multi-value return", multiret)
	// print("Closures", closures)
	// print("Finalizers", finalizers)
	// print("Defer stmts", deferstmt)
	// print("Go stmts", gostmt)
	// print("Interfaces", interfaces)
	// print("Type asserts", typeasserts)
	// printm("Imports", imports)
	print("Allocs", allocs)

}

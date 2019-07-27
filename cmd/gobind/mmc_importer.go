package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// mmcImporter is an importer that implements the `go/importer.Importer`
// interface. It imports package paths and handles packages that have opted
// into go modules and that have major version components as part of their
// imports. It tries to support the following logic from the
// "Minimum Module Compatibility" section of the go modules documentation:
//
//	* https://github.com/golang/go/wiki/Modules#how-are-v2-modules-treated-in-a-build-if-modules-support-is-not-enabled-how-does-minimal-module-compatibility-work-in-197-1103-and-111
//
// 		The entirety of the logic is – when operating in GOPATH mode, an
// 		unresolvable import statement containing a /vN will be tried again after
// 		removing the /vN if the import statement is inside code that has opted in
// 		to modules (that is, import statements in .go files within a tree with a
// 		valid go.mod file).
//
// It does not try to import the package twice but rather it rewrites in-memory any
// imports with `vN` components to not have such element and then loads the
// package using a `types.Checker`.
type mmcImporter struct {
	Ctx         build.Context
	stdImporter types.Importer
}

// Import tries to import the given package path and returns it's type information.
// `path` is _not_ an absolute file path but rather a package path used in an
// `import` statement.
func (imp *mmcImporter) Import(path string) (*types.Package, error) {

	if imp.stdImporter == nil {
		imp.stdImporter = importer.ForCompiler(token.NewFileSet(), "source", nil)
	}

	// try the std importer first (double import but easy to implement)
	if pkg, err := imp.stdImporter.Import(path); err == nil {
		return pkg, err
	}

	pkgSrcPath := filepath.Join(imp.Ctx.GOPATH, "src", path)

	fileSrcs, err := loadPackageSources(imp.Ctx, path, pkgSrcPath)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	var astFiles []*ast.File
	for fpath, src := range fileSrcs {
		astf, err := parser.ParseFile(fset, fpath, string(src), parser.ParseComments)
		if err != nil {
			return nil, err
		}

		// inspect the parsed files and replace any path imports with major versions
		// as non-versioned import paths.
		for _, imp := range astf.Imports {
			// TODO(tristian): Use the second return value to compute the new
			// position for the imp.Path token if possible.
			r, _ := removeMajorVersionFromPath(imp.Path.Value)

			// unsure if replacing the value will affect the `Pos` and `End`
			// properties of the import or how it will manifest
			// latter in the AST processing / checking.
			imp.Path.Value = r
		}

		astFiles = append(astFiles, astf)
	}

	conf := types.Config{
		Importer: imp,
	}

	return conf.Check(path, fset, astFiles, nil)
}

// loadPackageSources reads each of the Go file's sources for the given package
// under a specific build context. It respects the use of build tags to determine
// which files should be loaded.
func loadPackageSources(ctx build.Context, packagePath, srcDir string) (map[string][]byte, error) {

	pkgCnts, err := ctx.Import(packagePath, srcDir, 0)
	if err != nil {
		return nil, err
	}

	// produce absolute paths
	filePaths := make([]string, len(pkgCnts.GoFiles))
	for i := range pkgCnts.GoFiles {
		filePaths[i] = filepath.Join(srcDir, pkgCnts.GoFiles[i])
	}

	return loadFileContents(filePaths...)
}

// removeMajorVersionFromPath removes the `/vN` parts of a package path.
//
//	* "example.com/org/library/v6/pkg" -> "example.com/org/library/pkg"
//	* "example.com/org/library/v6" -> "example.com/org/library"
//
func removeMajorVersionFromPath(pkgPath string) (string, int) {
	var (
		nonVN    []string
		endQuote bool
		parts    = strings.Split(pkgPath, "/")
	)
	for i, p := range parts {
		if isVNPart(p) {
			// if the `/vN` was the last part of the path and it happened to end
			// with a `"` the closing quote will be lost when excluding the `/vN"`
			// string.
			endQuote = i == len(parts)-1 && strings.HasSuffix(p, `"`)
			continue
		}

		nonVN = append(nonVN, p)
	}

	r := strings.Join(nonVN, "/")
	if endQuote {
		r = r + `"`
	}
	return r, len(pkgPath) - len(r)
}

// isVNPart determines whether a string is the major version path component.
// It assumes that the string does not contain `"/"`, i.e it expects it to
// match `vN` or `vN"`
func isVNPart(s string) bool {
	var v int // just to hold the result of scanf
	// TODO(tristian): There's probably a cleaner way to check for `vN`
	_, err := fmt.Fscanf(strings.NewReader(s), "v%d", &v)
	return err == nil

}

// loadFileContents reads the content of each path and stores it into a map
// where the key is the path and the value the byte content.
func loadFileContents(fpath ...string) (map[string][]byte, error) {
	fileSrcs := make(map[string][]byte, len(fpath)) // path -> contents
	for _, fp := range fpath {
		d, err := ioutil.ReadFile(fp)
		if err != nil {
			return nil, err
		}

		fileSrcs[fp] = d
	}
	return fileSrcs, nil
}

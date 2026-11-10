package archrule

// Module is the syntax-level model of a Go module that the rules evaluate.
// The parser (an outbound adapter) fills it; the rules never touch files.
// Every path is module-relative and slash-separated; the module root is ".".
type Module struct {
	Path     string    `json:"path"`     // module path from go.mod
	Packages []Package `json:"packages"` // every directory holding .go files
	Dirs     []Dir     `json:"dirs"`     // every walked directory, for the cmd rule
	// Parser is the pack parser that built the model: "" or "go" for goparse,
	// "regex" for the import scanner. The Go-only rules (init, globals, cmd,
	// compose, parent imports) run only for Go.
	Parser string `json:"parser,omitempty"`
}

// IsGo reports whether the Go parser built the model.
func (m Module) IsGo() bool { return m.Parser == "" || m.Parser == "go" }

// Pos is a position in a module file.
type Pos struct {
	File string `json:"file"` // module-relative
	Line int    `json:"line"`
}

// Dir is one walked directory and what it holds.
type Dir struct {
	Path    string   `json:"path"`    // module-relative, "." for the root
	Entries []string `json:"entries"` // non-hidden entry names; sub-directories end in "/"
}

// Package is what the parser gathers for one directory of .go files.
// Test files (_test.go) feed only Globals and TestFiles.
type Package struct {
	Dir        string    `json:"dir"`        // module-relative, "." for the root
	ImportPath string    `json:"importPath"` // Module.Path + "/" + Dir
	Files      []string  `json:"files"`      // non-test .go files, module-relative
	TestFiles  []string  `json:"testFiles"`  // _test.go files, module-relative
	Lines      int       `json:"lines"`      // newlines over Files
	Imports    []Import  `json:"imports"`    // one per import spec in Files
	Inits      []Pos     `json:"inits"`      // func init() declarations in Files
	Globals    []Global  `json:"globals"`    // package vars some function body assigns
	Types      []Type    `json:"types"`      // top-level types and receiver types of Files
	GoStmts    []Pos     `json:"goStmts"`    // go statements in Files
	Calls      []Call    `json:"calls"`      // time.Now and math/rand(/v2) selectors in Files
	Main       *MainFile `json:"main"`       // <Dir>/main.go facts, nil without one
	// Impure holds the impure-call text hits of the import scanner (non-Go
	// packs); Evaluate reports them in domain packages.
	Impure []Hit `json:"impure,omitempty"`
}

// Hit is one impure-call entry found on a source line.
type Hit struct {
	Call string `json:"call"` // the pack's impure_calls entry
	Pos  Pos    `json:"pos"`
}

// Import is one import spec.
type Import struct {
	Path string `json:"path"`
	Pos  Pos    `json:"pos"`
	// Kind is set by non-Go parsers, whose resolver already classified the
	// import: ImportInternal (Dir holds the target), ImportStd or
	// ImportThird. Empty means Go: the module path decides.
	Kind string `json:"kind,omitempty"`
	Dir  string `json:"dir,omitempty"` // module-relative target directory of an internal import
}

// Import kinds a resolver assigns.
const (
	ImportInternal = "internal"
	ImportStd      = "std"
	ImportThird    = "third"
)

// Global is a package-level var (declared in a non-test file) that a
// function body, test files included, assigns to or increments.
type Global struct {
	Name     string `json:"name"`
	Decl     Pos    `json:"decl"`
	Assigned Pos    `json:"assigned"` // the first assignment found
}

// Type kinds.
const (
	KindStruct    = "struct"
	KindInterface = "interface"
	KindOther     = "other"
)

// Type is one named type of a package.
type Type struct {
	Name         string `json:"name"`
	Pos          Pos    `json:"pos"`          // its declaration, or its first method when undeclared
	Kind         string `json:"kind"`         // KindStruct, KindInterface, KindOther, "" when undeclared
	Methods      int    `json:"methods"`      // methods declared with it as receiver
	Fields       int    `json:"fields"`       // struct fields; an embedded field counts 1
	IfaceMethods int    `json:"ifaceMethods"` // interface methods; an embedded interface counts 1
}

// Call is a use of a selected package identifier: time.Now, or any
// math/rand or math/rand/v2 selector (called, taken as a value, or a type).
type Call struct {
	Pkg  string `json:"pkg"`  // import path: "time", "math/rand", "math/rand/v2"
	Name string `json:"name"` // the selected identifier
	Pos  Pos    `json:"pos"`
}

// MainFile is what the cmd rule needs from a main.go.
type MainFile struct {
	File    string      `json:"file"`
	Lines   int         `json:"lines"`   // newlines in the file
	Decls   []Fact      `json:"decls"`   // top-level declarations except imports: "func main", "func help", "method run", "var declaration"
	Logic   []Fact      `json:"logic"`   // "for/range statement", "switch statement", "select statement", "go statement", "function literal"
	Strings []StringLit `json:"strings"` // string literals outside import specs
}

// Fact is a named thing at a position.
type Fact struct {
	What string `json:"what"`
	Pos  Pos    `json:"pos"`
}

// StringLit is a string literal and its length in runes, quotes excluded.
type StringLit struct {
	Runes int `json:"runes"`
	Pos   Pos `json:"pos"`
}

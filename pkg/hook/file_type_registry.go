// Package hook provides file type detection and hook execution functionality.
package hook

import (
	"path/filepath"
	"slices"
	"strings"
)

const (
	fileTypeDockerfile = "dockerfile"
	fileTypeRuby       = "ruby"
	extHTML            = ".html"
	extTS              = ".ts"
	extJS              = ".js"
	extTSX             = ".tsx"
	extYAML            = ".yaml"
)

// FileTypeRegistry consolidates all file type matching logic
type FileTypeRegistry struct {
	typeMap map[string][]string
}

// NewFileTypeRegistry creates a new file type registry with predefined mappings
func NewFileTypeRegistry() *FileTypeRegistry {
	registry := &FileTypeRegistry{
		typeMap: map[string][]string{
			"python":        {".py", ".pyx", ".pyi"},
			"javascript":    {".js", ".jsx", ".mjs"},
			"typescript":    {".ts", ".tsx"},
			"yaml":          {".yaml", ".yml"},
			"json":          {".json"},
			"markdown":      {".md", ".markdown", ".mdown", ".mkd"},
			"go":            {".go"},
			"shell":         {".sh", ".bash", ".zsh", ".fish"},
			"css":           {".css", ".scss", ".sass", ".less"},
			"html":          {".html", ".htm", ".xhtml"},
			"xml":           {".xml", ".xsl", ".xsd"},
			"toml":          {".toml"},
			"ini":           {".ini", ".cfg"},
			"rust":          {".rs"},
			"java":          {".java"},
			"c":             {".c", ".h"},
			"cpp":           {".cpp", ".cc", ".cxx", ".hpp", ".hxx"},
			"ruby":          {".rb"},
			"php":           {".php", ".phtml"},
			"perl":          {".pl", ".pm"},
			"swift":         {".swift"},
			"kotlin":        {".kt", ".kts"},
			"scala":         {".scala"},
			"r":             {".r", ".rmd"},
			"tex":           {".tex", ".latex"},
			"sql":           {".sql"},
			"dart":          {".dart"},
			"haskell":       {".hs", ".lhs"},
			"elixir":        {".ex", ".exs"},
			"elm":           {".elm"},
			"erlang":        {".erl"},
			"fsharp":        {".fs", ".fsx", ".fsi"},
			"csharp":        {".cs"},
			"vb":            {".vb"},
			"powershell":    {".ps1"},
			"lua":           {".lua"},
			"clojure":       {".clj", ".cljs", ".cljc"},
			"groovy":        {".groovy", ".gvy"},
			"matlab":        {".m"},
			"julia":         {".jl"},
			"vim":           {".vim"},
			"tcl":           {".tcl"},
			"ada":           {".ada", ".adb", ".ads"},
			"fortran":       {".f", ".for", ".f90", ".f95", ".f03", ".f08"},
			"cobol":         {".cob", ".cbl", ".cpy"},
			"pascal":        {".pas", ".pp"},
			"prolog":        {".pl", ".pro"},
			"lisp":          {".lisp", ".lsp", ".cl"},
			"scheme":        {".scm", ".ss"},
			"racket":        {".rkt"},
			"ocaml":         {".ml", ".mli"},
			"reasonml":      {".re", ".rei"},
			"crystal":       {".cr"},
			"nim":           {".nim"},
			"zig":           {".zig"},
			"d":             {".d"},
			"vhdl":          {".vhd", ".vhdl"},
			"verilog":       {".v", ".vh"},
			"systemverilog": {".sv", ".svh"},
			"terraform":     {".tf", ".tfvars"},
			"vue":           {".vue"},
			"svelte":        {".svelte"},
			"react":         {".jsx", ".tsx"},
			"jinja":         {".j2", ".jinja", ".jinja2"},
			"handlebars":    {".hbs", ".handlebars"},
			"mustache":      {".mustache"},
			"liquid":        {".liquid"},
			"smarty":        {".tpl"},
			"nunjucks":      {".njk"},
		},
	}
	return registry
}

// MatchesType checks if a file matches the given type
func (ftr *FileTypeRegistry) MatchesType(file, fileType string) bool {
	ext := strings.ToLower(filepath.Ext(file))
	fileName := strings.ToLower(filepath.Base(file))

	// Handle special cases first
	if ftr.matchesSpecialType(file, fileType, fileName) {
		return true
	}

	// Check extension mappings
	return ftr.matchesExtension(fileType, ext)
}

// matchesSpecialType handles special file type cases
func (ftr *FileTypeRegistry) matchesSpecialType(file, fileType, fileName string) bool {
	switch fileType {
	case "text":
		return ftr.isTextFile(file)
	case fileTypeDockerfile:
		return fileName == fileTypeDockerfile || strings.HasPrefix(fileName, fileTypeDockerfile+".")
	case fileTypeRuby:
		return fileName == "gemfile" || fileName == "rakefile"
	case "helm":
		return fileName == "chart.yaml" || strings.Contains(file, "templates/")
	case "docker-compose":
		return ftr.matchesDockerCompose(fileName)
	case "vagrant":
		return fileName == "vagrantfile"
	case "django", "flask":
		return ftr.matchesHTMLTemplate(file)
	case "angular":
		return ftr.matchesAngular(file)
	}
	return false
}

// matchesDockerCompose checks if file matches docker-compose patterns
func (ftr *FileTypeRegistry) matchesDockerCompose(fileName string) bool {
	return fileName == "docker-compose.yml" || fileName == "docker-compose.yaml" ||
		strings.HasPrefix(fileName, "docker-compose.") || strings.HasPrefix(fileName, "compose.")
}

// matchesHTMLTemplate checks if file is an HTML template
func (ftr *FileTypeRegistry) matchesHTMLTemplate(file string) bool {
	ext := strings.ToLower(filepath.Ext(file))
	return ext == extHTML && strings.Contains(file, "templates/")
}

// matchesAngular checks if file is an Angular component/service/module
func (ftr *FileTypeRegistry) matchesAngular(file string) bool {
	ext := strings.ToLower(filepath.Ext(file))
	return ext == extTS && (strings.Contains(file, ".component.") ||
		strings.Contains(file, ".service.") || strings.Contains(file, ".module."))
}

// matchesExtension checks if file extension matches the type
func (ftr *FileTypeRegistry) matchesExtension(fileType, ext string) bool {
	if extensions, exists := ftr.typeMap[fileType]; exists {
		if slices.Contains(extensions, ext) {
			return true
		}
	}
	return false
}

// MatchesAnyType checks if file matches any of the given types (OR logic)
func (ftr *FileTypeRegistry) MatchesAnyType(file string, types []string) bool {
	for _, typ := range types {
		if ftr.MatchesType(file, typ) {
			return true
		}
	}
	return false
}

// MatchesAllTypes checks if file matches ALL of the given types (AND logic)
func (ftr *FileTypeRegistry) MatchesAllTypes(file string, types []string) bool {
	for _, typ := range types {
		if !ftr.MatchesType(file, typ) {
			return false
		}
	}
	return true
}

// isTextFile checks if a file is likely a text file
func (ftr *FileTypeRegistry) isTextFile(file string) bool {
	binaryExts := []string{
		".exe",
		".bin",
		".so",
		".dll",
		".png",
		".jpg",
		".jpeg",
		".gif",
		".pdf",
		".zip",
		".tar",
		".gz",
	}
	ext := strings.ToLower(filepath.Ext(file))

	return !slices.Contains(binaryExts, ext)
}

// AddCustomType allows adding custom file type mappings
func (ftr *FileTypeRegistry) AddCustomType(typeName string, extensions []string) {
	ftr.typeMap[typeName] = extensions
}

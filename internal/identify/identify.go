// Package identify provides file type identification by extension, filename, and shebang.
package identify

import (
	"os"
	"path/filepath"
	"strings"
)

// TagsForFile returns the set of type tags for a file path.
func TagsForFile(path string) map[string]bool {
	tags := make(map[string]bool)

	// Always add "file".
	tags["file"] = true

	// Check if binary.
	if isBinaryFile(path) {
		tags["binary"] = true
		return tags
	}
	tags["text"] = true

	// Add extension-based tags.
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		ext = ext[1:] // Remove leading dot.
		if mapped, ok := extensionMap[ext]; ok {
			for _, t := range mapped {
				tags[t] = true
			}
		}
	}

	// Add filename-based tags.
	base := strings.ToLower(filepath.Base(path))
	if mapped, ok := filenameMap[base]; ok {
		for _, t := range mapped {
			tags[t] = true
		}
	}

	// Check shebang.
	if shebangTags := getShebangTags(path); len(shebangTags) > 0 {
		for _, t := range shebangTags {
			tags[t] = true
		}
	}

	return tags
}

// MatchesTypes checks if a set of tags satisfies type filters.
// types are ANDed: all must match.
// typesOr are ORed: at least one must match.
// excludeTypes: none must match.
func MatchesTypes(tags map[string]bool, types []string, typesOr []string, excludeTypes []string) bool {
	// Check exclude types first.
	for _, t := range excludeTypes {
		if tags[t] {
			return false
		}
	}

	// Check types (AND).
	for _, t := range types {
		if !tags[t] {
			return false
		}
	}

	// Check types_or (OR). If specified, at least one must match.
	if len(typesOr) > 0 {
		found := false
		for _, t := range typesOr {
			if tags[t] {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if n == 0 {
		return false
	}
	buf = buf[:n]

	// Check for null bytes (common indicator of binary).
	for _, b := range buf {
		if b == 0 {
			return true
		}
	}
	return false
}

func getShebangTags(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	buf := make([]byte, 256)
	n, _ := f.Read(buf)
	if n < 2 {
		return nil
	}
	line := string(buf[:n])
	if !strings.HasPrefix(line, "#!") {
		return nil
	}

	// Get first line.
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	line = strings.TrimSpace(line[2:])

	// Handle /usr/bin/env
	if strings.HasPrefix(line, "/usr/bin/env ") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			line = parts[1]
		}
	}

	// Get the basename of the interpreter.
	interp := filepath.Base(line)

	// Strip version numbers.
	for _, lang := range []string{"python", "ruby", "perl", "node", "bash", "sh", "zsh", "fish"} {
		if strings.HasPrefix(interp, lang) {
			return shebangInterpreters[lang]
		}
	}

	if tags, ok := shebangInterpreters[interp]; ok {
		return tags
	}

	return nil
}

// shebangInterpreters maps interpreter names to tags.
var shebangInterpreters = map[string][]string{
	"python":  {"python"},
	"python3": {"python", "python3"},
	"ruby":    {"ruby"},
	"node":    {"javascript"},
	"bash":    {"bash", "shell"},
	"sh":      {"sh", "shell"},
	"zsh":     {"zsh", "shell"},
	"fish":    {"fish", "shell"},
	"perl":    {"perl"},
	"php":     {"php"},
	"lua":     {"lua"},
	"Rscript": {"r"},
}

// extensionMap maps file extensions to type tags.
var extensionMap = map[string][]string{
	// Python
	"py":    {"python"},
	"pyi":   {"python", "pyi"},
	"pyx":   {"python", "cython"},
	"pxd":   {"python", "cython"},
	"pyw":   {"python"},

	// JavaScript / TypeScript
	"js":    {"javascript"},
	"jsx":   {"javascript", "jsx"},
	"ts":    {"typescript"},
	"tsx":   {"typescript", "tsx"},
	"mjs":   {"javascript"},
	"cjs":   {"javascript"},
	"mts":   {"typescript"},
	"cts":   {"typescript"},

	// Ruby
	"rb":       {"ruby"},
	"erb":      {"ruby", "erb"},
	"gemspec":  {"ruby"},
	"rake":     {"ruby"},
	"ru":       {"ruby"},

	// Go
	"go":   {"go"},
	"mod":  {"go-mod"},
	"sum":  {"go-sum"},
	"tmpl": {"go-template"},

	// Rust
	"rs":   {"rust"},

	// Java / JVM
	"java":    {"java"},
	"class":   {"java", "binary"},
	"jar":     {"java", "binary"},
	"kt":      {"kotlin"},
	"kts":     {"kotlin"},
	"scala":   {"scala"},
	"groovy":  {"groovy"},
	"gradle":  {"groovy", "gradle"},
	"clj":     {"clojure"},
	"cljs":    {"clojurescript"},
	"cljc":    {"clojure"},
	"edn":     {"clojure", "edn"},

	// C / C++ / ObjC
	"c":     {"c"},
	"h":     {"c", "header"},
	"cpp":   {"c++"},
	"cc":    {"c++"},
	"cxx":   {"c++"},
	"c++":   {"c++"},
	"hpp":   {"c++", "header"},
	"hxx":   {"c++", "header"},
	"hh":    {"c++", "header"},
	"inl":   {"c++"},
	"ipp":   {"c++"},
	"tcc":   {"c++"},
	"cs":    {"c#"},
	"csx":   {"c#"},
	"swift": {"swift"},
	"m":     {"objective-c"},
	"mm":    {"objective-c++"},

	// Perl
	"pl":    {"perl"},
	"pm":    {"perl"},
	"pod":   {"perl", "pod"},
	"t":     {"perl"},

	// R
	"r":     {"r"},
	"R":     {"r"},
	"rmd":   {"r", "markdown"},
	"Rmd":   {"r", "markdown"},
	"rnw":   {"r"},
	"Rnw":   {"r"},

	// Lua
	"lua":   {"lua"},
	"luau":  {"lua"},

	// Shell
	"sh":      {"shell", "bash"},
	"bash":    {"shell", "bash"},
	"zsh":     {"shell", "zsh"},
	"fish":    {"shell", "fish"},
	"ksh":     {"shell", "ksh"},
	"csh":     {"shell", "csh"},
	"tcsh":    {"shell", "tcsh"},
	"ash":     {"shell", "ash"},
	"dash":    {"shell"},
	"bats":    {"shell", "bash", "bats"},
	"ps1":     {"powershell"},
	"psm1":    {"powershell"},
	"psd1":    {"powershell"},
	"bat":     {"batch"},
	"cmd":     {"batch"},

	// PHP
	"php":   {"php"},
	"phtml": {"php"},
	"php3":  {"php"},
	"php4":  {"php"},
	"php5":  {"php"},
	"phps":  {"php"},

	// Markup and data formats
	"json":    {"json"},
	"jsonl":   {"json"},
	"json5":   {"json5"},
	"geojson": {"json", "geojson"},
	"yaml":    {"yaml"},
	"yml":     {"yaml"},
	"toml":    {"toml"},
	"xml":     {"xml"},
	"xsl":     {"xml", "xsl"},
	"xslt":    {"xml", "xsl"},
	"xsd":     {"xml", "xsd"},
	"dtd":     {"xml", "dtd"},
	"plist":   {"xml", "plist"},
	"html":    {"html"},
	"htm":     {"html"},
	"xhtml":   {"html", "xhtml"},
	"vue":     {"vue"},
	"svelte":  {"svelte"},
	"css":     {"css"},
	"scss":    {"scss", "css"},
	"sass":    {"sass", "css"},
	"less":    {"less", "css"},
	"styl":    {"stylus", "css"},
	"md":      {"markdown"},
	"mdx":     {"markdown", "mdx"},
	"markdown":{"markdown"},
	"rst":     {"rst"},
	"tex":     {"tex", "latex"},
	"latex":   {"tex", "latex"},
	"bib":     {"bib", "latex"},
	"csv":     {"csv"},
	"tsv":     {"tsv"},

	// Config files
	"ini":         {"ini"},
	"cfg":         {"ini"},
	"conf":        {"conf"},
	"properties":  {"properties", "java-properties"},
	"env":         {"dotenv"},
	"envrc":       {"dotenv"},
	"gitconfig":   {"gitconfig"},

	// Images
	"png":   {"image", "png"},
	"jpg":   {"image", "jpeg"},
	"jpeg":  {"image", "jpeg"},
	"gif":   {"image", "gif"},
	"bmp":   {"image", "bmp"},
	"tiff":  {"image", "tiff"},
	"tif":   {"image", "tiff"},
	"webp":  {"image", "webp"},
	"svg":   {"image", "svg"},
	"ico":   {"image", "icon"},
	"icns":  {"image", "icon"},
	"psd":   {"image", "psd"},
	"ai":    {"image", "ai"},
	"eps":   {"image", "eps"},

	// Audio/Video
	"mp3":   {"audio"},
	"mp4":   {"video"},
	"avi":   {"video"},
	"mkv":   {"video"},
	"mov":   {"video"},
	"wav":   {"audio"},
	"flac":  {"audio"},
	"ogg":   {"audio"},
	"webm":  {"video"},

	// Fonts
	"ttf":    {"font"},
	"otf":    {"font"},
	"woff":   {"font"},
	"woff2":  {"font"},
	"eot":    {"font"},

	// Documents
	"pdf":    {"pdf"},
	"doc":    {"document"},
	"docx":   {"document"},
	"xls":    {"document"},
	"xlsx":   {"document"},
	"ppt":    {"document"},
	"pptx":   {"document"},
	"odt":    {"document"},
	"ods":    {"document"},
	"odp":    {"document"},
	"rtf":    {"document", "rtf"},

	// Build, package, and DevOps
	"sql":     {"sql"},
	"tf":      {"terraform", "hcl"},
	"tfvars":  {"terraform", "hcl"},
	"hcl":     {"hcl"},
	"proto":   {"protobuf"},
	"thrift":  {"thrift"},
	"avro":    {"avro"},
	"graphql": {"graphql"},
	"gql":     {"graphql"},

	// Haskell
	"hs":     {"haskell"},
	"lhs":    {"haskell"},
	"cabal":  {"haskell", "cabal"},

	// Elixir / Erlang
	"ex":     {"elixir"},
	"exs":    {"elixir"},
	"erl":    {"erlang"},
	"hrl":    {"erlang"},

	// Dart / Flutter
	"dart":   {"dart"},

	// Zig
	"zig":    {"zig"},

	// Nim
	"nim":    {"nim"},
	"nimble": {"nim"},

	// Julia
	"jl":     {"julia"},

	// F# / OCaml / ML
	"fs":     {"f#"},
	"fsx":    {"f#"},
	"fsi":    {"f#"},
	"ml":     {"ocaml"},
	"mli":    {"ocaml"},
	"re":     {"reason"},
	"rei":    {"reason"},

	// Lisp family
	"el":     {"emacs-lisp", "lisp"},
	"lisp":   {"lisp"},
	"lsp":    {"lisp"},
	"scm":    {"scheme", "lisp"},
	"rkt":    {"racket", "lisp"},
	"hy":     {"hy", "lisp"},

	// Assembly
	"asm":    {"assembly"},
	"s":      {"assembly"},
	"S":      {"assembly"},
	"nasm":   {"assembly"},

	// Fortran
	"f":      {"fortran"},
	"f90":    {"fortran"},
	"f95":    {"fortran"},
	"f03":    {"fortran"},
	"for":    {"fortran"},

	// D
	"d":      {"dlang"},
	"di":     {"dlang"},

	// V
	"v":      {"vlang"},

	// Crystal
	"cr":     {"crystal"},

	// Templates
	"j2":       {"jinja"},
	"jinja":    {"jinja"},
	"jinja2":   {"jinja"},
	"twig":     {"twig"},
	"ejs":      {"ejs"},
	"hbs":      {"handlebars"},
	"mustache": {"mustache"},
	"pug":      {"pug"},
	"jade":     {"pug"},
	"slim":     {"slim"},
	"haml":     {"haml"},

	// Config / Schema
	"jsonschema": {"json", "jsonschema"},
	"avsc":       {"json", "avro"},
	"prisma":     {"prisma"},
	"nginx":      {"nginx"},

	// Notebooks / Data
	"ipynb":  {"jupyter"},
	"rdata":  {"r"},
	"rds":    {"r"},
	"sas":    {"sas"},
	"sps":    {"spss"},
	"do":     {"stata"},
	"dta":    {"stata"},
	"mat":    {"matlab"},

	// Makefile-like
	"mk":     {"makefile"},
	"cmake":  {"cmake"},

	// Docker / CI
	"dockerignore": {"dockerignore"},

	// Nix
	"nix":    {"nix"},

	// Cue
	"cue":    {"cue"},

	// Jsonnet
	"jsonnet":  {"jsonnet"},
	"libsonnet": {"jsonnet"},

	// WASM
	"wasm":   {"wasm", "binary"},
	"wat":    {"wasm-text"},

	// Certificates / Keys
	"pem":    {"pem"},
	"crt":    {"pem", "certificate"},
	"key":    {"pem", "key"},
	"cer":    {"certificate"},
	"p12":    {"certificate", "binary"},
	"pfx":    {"certificate", "binary"},

	// Archives
	"zip":    {"archive", "binary"},
	"tar":    {"archive", "binary"},
	"gz":     {"archive", "binary"},
	"bz2":    {"archive", "binary"},
	"xz":     {"archive", "binary"},
	"7z":     {"archive", "binary"},
	"rar":    {"archive", "binary"},
	"tgz":    {"archive", "binary"},

	// Misc
	"diff":   {"diff"},
	"patch":  {"diff", "patch"},
	"log":    {"log"},
	"lock":   {"lock"},
	"pid":    {"pid"},
	"map":    {"sourcemap"},
	"wsdl":   {"xml", "wsdl"},
}

// filenameMap maps specific filenames to type tags.
var filenameMap = map[string][]string{
	"makefile":         {"makefile"},
	"gnumakefile":      {"makefile"},
	"dockerfile":       {"dockerfile"},
	"containerfile":    {"dockerfile"},
	"vagrantfile":      {"ruby"},
	"gemfile":          {"ruby"},
	"gemfile.lock":     {"ruby", "lock"},
	"rakefile":         {"ruby"},
	"cmakelists.txt":   {"cmake"},
	"jenkinsfile":      {"groovy"},
	"procfile":         {"procfile"},
	"brewfile":         {"ruby"},

	// Git
	".gitignore":       {"gitignore"},
	".gitattributes":   {"gitattributes"},
	".gitmodules":      {"gitmodules"},
	".gitkeep":         {"gitkeep"},
	".mailmap":         {"mailmap"},

	// Editor
	".editorconfig":    {"editorconfig"},
	".prettierrc":      {"json", "prettier"},
	".prettierignore":  {"prettier"},
	".eslintrc":        {"json", "eslint"},
	".eslintrc.json":   {"json", "eslint"},
	".eslintrc.yml":    {"yaml", "eslint"},
	".eslintrc.yaml":   {"yaml", "eslint"},
	".eslintrc.js":     {"javascript", "eslint"},
	".eslintignore":    {"eslint"},
	".stylelintrc":     {"json"},
	".babelrc":         {"json"},
	".browserslistrc":  {"browserslist"},

	// Package managers
	"package.json":     {"json", "package-json"},
	"package-lock.json":{"json", "package-json", "lock"},
	"tsconfig.json":    {"json", "tsconfig"},
	"yarn.lock":        {"yaml", "lock"},
	"pnpm-lock.yaml":   {"yaml", "lock"},
	"composer.json":    {"json", "composer"},
	"composer.lock":    {"json", "lock"},
	"cargo.toml":       {"toml", "cargo"},
	"cargo.lock":       {"toml", "lock"},
	"go.mod":           {"go-mod"},
	"go.sum":           {"go-sum"},
	"pubspec.yaml":     {"yaml", "dart"},
	"pubspec.lock":     {"yaml", "dart", "lock"},
	"requirements.txt": {"requirements-txt", "pip"},
	"setup.py":         {"python", "setuptools"},
	"setup.cfg":        {"ini", "setuptools"},
	"pyproject.toml":   {"toml", "python"},
	"pipfile":          {"toml", "pip"},
	"pipfile.lock":     {"json", "pip", "lock"},
	"poetry.lock":      {"toml", "lock"},

	// CI
	".travis.yml":      {"yaml", "travis"},
	".circleci/config.yml": {"yaml", "circleci"},
	"appveyor.yml":     {"yaml", "appveyor"},
	"azure-pipelines.yml": {"yaml", "azure"},
	".github/dependabot.yml": {"yaml", "dependabot"},
	"netlify.toml":     {"toml", "netlify"},
	"vercel.json":      {"json", "vercel"},
	"render.yaml":      {"yaml"},

	// Docker
	"docker-compose.yml":  {"yaml", "docker-compose"},
	"docker-compose.yaml": {"yaml", "docker-compose"},
	".dockerignore":       {"dockerignore"},

	// Config files
	".pre-commit-config.yaml": {"yaml", "pre-commit"},
	".pre-commit-hooks.yaml":  {"yaml", "pre-commit"},
	".flake8":          {"ini", "flake8"},
	".pylintrc":        {"ini", "pylint"},
	".rubocop.yml":     {"yaml", "rubocop"},
	".yamllint":        {"yaml"},
	".yamllint.yml":    {"yaml"},
	".yamllint.yaml":   {"yaml"},
	".markdownlint.yml":{"yaml"},
	".clang-format":    {"yaml", "clang-format"},
	".clang-tidy":      {"yaml"},
	"tox.ini":          {"ini", "tox"},
	"mypy.ini":         {"ini", "mypy"},
	".mypy.ini":        {"ini", "mypy"},
	".coveragerc":      {"ini", "coverage"},
	".isort.cfg":       {"ini", "isort"},
	"rustfmt.toml":     {"toml", "rustfmt"},
	".rustfmt.toml":    {"toml", "rustfmt"},
	"clippy.toml":      {"toml", "clippy"},
	".clippy.toml":     {"toml", "clippy"},
	".golangci.yml":    {"yaml", "golangci"},
	".golangci.yaml":   {"yaml", "golangci"},
	".goreleaser.yml":  {"yaml", "goreleaser"},
	".goreleaser.yaml": {"yaml", "goreleaser"},

	// Shell
	".bashrc":          {"shell", "bash"},
	".bash_profile":    {"shell", "bash"},
	".bash_logout":     {"shell", "bash"},
	".zshrc":           {"shell", "zsh"},
	".zshenv":          {"shell", "zsh"},
	".zprofile":        {"shell", "zsh"},
	".profile":         {"shell"},
	".inputrc":         {"inputrc"},

	// Env
	".env":             {"dotenv"},
	".env.example":     {"dotenv"},
	".env.local":       {"dotenv"},
	".env.development": {"dotenv"},
	".env.production":  {"dotenv"},
	".env.test":        {"dotenv"},
	".envrc":           {"dotenv", "direnv"},

	// License
	"license":          {"license"},
	"licence":          {"license"},
	"license.md":       {"license", "markdown"},
	"licence.md":       {"license", "markdown"},
	"license.txt":      {"license"},
	"licence.txt":      {"license"},
	"copying":          {"license"},

	// Docs
	"readme":           {"readme"},
	"readme.md":        {"readme", "markdown"},
	"readme.rst":       {"readme", "rst"},
	"readme.txt":       {"readme"},
	"changelog":        {"changelog"},
	"changelog.md":     {"changelog", "markdown"},
	"changes":          {"changelog"},
	"authors":          {"authors"},
	"contributors":     {"authors"},
	"codeowners":       {"codeowners"},
	".codeowners":      {"codeowners"},
}

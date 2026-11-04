// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

// Package identify provides file type identification by extension, filename, and shebang.
//
// The tag tables below follow the data in the MIT-licensed identify project
// (https://github.com/pre-commit/identify, © 2017 Chris Kuehl, Anthony Sottile).
// A hook's types/types_or/exclude_types filters are matched against these tags,
// so a tag this project gets wrong is a hook that silently skips or wrongly
// claims a file: when the two disagree, upstream is right. See NOTICE.
package identify

import (
	"os"
	"path/filepath"
	"strings"
)

// The tags a path gets from what it is rather than from its name: upstream
// identify's TYPE_TAGS and ENCODING_TAGS, under its own names.
const (
	TagDirectory = "directory"
	TagSymlink   = "symlink"
	TagSocket    = "socket"
	TagFile      = "file"
	TagText      = "text"
	TagBinary    = "binary"
)

// TagsForFile returns the set of type tags for a file path.
//
// Non-regular files get exactly one tag and nothing else, matching upstream
// identify's tags_from_path. This is what keeps them out of hooks: the default
// filter is `types: [file]`, and a directory tagged only "directory" cannot
// match it. Tagging them "file" instead hands a git submodule's path to hooks
// that then try to open it as a file.
func TagsForFile(path string) map[string]bool {
	if info, err := os.Lstat(path); err == nil {
		switch mode := info.Mode(); {
		case mode.IsDir():
			return map[string]bool{TagDirectory: true}
		case mode&os.ModeSymlink != 0:
			return map[string]bool{TagSymlink: true}
		case mode&os.ModeSocket != 0:
			return map[string]bool{TagSocket: true}
		}
	}

	tags := make(map[string]bool)

	// Always add "file".
	tags[TagFile] = true

	// Check if binary.
	if isBinaryFile(path) {
		tags[TagBinary] = true
		return tags
	}
	tags[TagText] = true

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
func MatchesTypes(tags map[string]bool, types, typesOr, excludeTypes []string) bool {
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
	n, _ := f.Read(buf)
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
	for _, lang := range []string{
		interpPython, interpRuby, interpPerl, interpNode, interpBash, interpSh, interpZsh, interpFish,
	} {
		if strings.HasPrefix(interp, lang) {
			return shebangInterpreters[lang]
		}
	}

	if tags, ok := shebangInterpreters[interp]; ok {
		return tags
	}

	return nil
}

// The interpreters getShebangTags matches by prefix, so that a versioned
// name such as python3.12 finds its entry in shebangInterpreters.
const (
	interpPython = "python"
	interpRuby   = "ruby"
	interpPerl   = "perl"
	interpNode   = "node"
	interpBash   = "bash"
	interpSh     = "sh"
	interpZsh    = "zsh"
	interpFish   = "fish"
)

// shebangInterpreters maps interpreter names to tags.
var shebangInterpreters = map[string][]string{
	interpPython: {tagPython},
	"python3":    {tagPython, "python3"},
	interpRuby:   {tagRuby},
	interpNode:   {tagJavaScript},
	interpBash:   {tagBash, tagShell},
	interpSh:     {"sh", tagShell},
	interpZsh:    {tagZsh, tagShell},
	interpFish:   {tagFish, tagShell},
	interpPerl:   {tagPerl},
	"php":        {tagPHP},
	"lua":        {tagLua},
	"Rscript":    {tagR},
}

// Tags that more than one entry below shares. A tag only one entry uses stays
// a literal beside its extension or filename.
const (
	tagArchive       = "archive"
	tagAssembly      = "assembly"
	tagAudio         = "audio"
	tagAuthors       = "authors"
	tagAvro          = "avro"
	tagBash          = "bash"
	tagBatch         = "batch"
	tagC             = "c"
	tagCSharp        = "c#"
	tagCPlusPlus     = "c++"
	tagCertificate   = "certificate"
	tagChangelog     = "changelog"
	tagClippy        = "clippy"
	tagClojure       = "clojure"
	tagCMake         = "cmake"
	tagCodeowners    = "codeowners"
	tagCSS           = "css"
	tagCython        = "cython"
	tagDart          = "dart"
	tagDiff          = "diff"
	tagDlang         = "dlang"
	tagDockerCompose = "docker-compose"
	tagDockerfile    = "dockerfile"
	tagDockerignore  = "dockerignore"
	tagDocument      = "document"
	tagDotenv        = "dotenv"
	tagElixir        = "elixir"
	tagErlang        = "erlang"
	tagESLint        = "eslint"
	tagFSharp        = "f#"
	tagFish          = "fish"
	tagFont          = "font"
	tagFortran       = "fortran"
	tagGoMod         = "go-mod"
	tagGoSum         = "go-sum"
	tagGolangci      = "golangci"
	tagGoReleaser    = "goreleaser"
	tagGraphQL       = "graphql"
	tagGroovy        = "groovy"
	tagHaskell       = "haskell"
	tagHCL           = "hcl"
	tagHeader        = "header"
	tagHTML          = "html"
	tagIcon          = "icon"
	tagImage         = "image"
	tagINI           = "ini"
	tagJava          = "java"
	tagJavaScript    = "javascript"
	tagJinja         = "jinja"
	tagJPEG          = "jpeg"
	tagJSON          = "json"
	tagJsonnet       = "jsonnet"
	tagKotlin        = "kotlin"
	tagLaTeX         = "latex"
	tagLicense       = "license"
	tagLisp          = "lisp"
	tagLock          = "lock"
	tagLua           = "lua"
	tagMakefile      = "makefile"
	tagMarkdown      = "markdown"
	tagMypy          = "mypy"
	tagNim           = "nim"
	tagOCaml         = "ocaml"
	tagPackageJSON   = "package-json"
	tagPEM           = "pem"
	tagPerl          = "perl"
	tagPHP           = "php"
	tagPip           = "pip"
	tagPowerShell    = "powershell"
	tagPreCommit     = "pre-commit"
	tagPrettier      = "prettier"
	tagPug           = "pug"
	tagPython        = "python"
	tagR             = "r"
	tagReadme        = "readme"
	tagReason        = "reason"
	tagRST           = "rst"
	tagRuby          = "ruby"
	tagRustfmt       = "rustfmt"
	tagSetuptools    = "setuptools"
	tagShell         = "shell"
	tagStata         = "stata"
	tagTerraform     = "terraform"
	tagTeX           = "tex"
	tagTIFF          = "tiff"
	tagTOML          = "toml"
	tagTypeScript    = "typescript"
	tagVideo         = "video"
	tagXML           = "xml"
	tagXSL           = "xsl"
	tagYAML          = "yaml"
	tagZsh           = "zsh"
)

// extensionMap maps file extensions to type tags.
var extensionMap = map[string][]string{
	// Python
	"py":  {tagPython},
	"pyi": {tagPython, "pyi"},
	"pyx": {tagPython, tagCython},
	"pxd": {tagPython, tagCython},
	"pyw": {tagPython},

	// JavaScript / TypeScript
	"js":  {tagJavaScript},
	"jsx": {tagJavaScript, "jsx"},
	"ts":  {tagTypeScript},
	"tsx": {tagTypeScript, "tsx"},
	"mjs": {tagJavaScript},
	"cjs": {tagJavaScript},
	"mts": {tagTypeScript},
	"cts": {tagTypeScript},

	// Ruby
	"rb":      {tagRuby},
	"erb":     {tagRuby, "erb"},
	"gemspec": {tagRuby},
	"rake":    {tagRuby},
	"ru":      {tagRuby},

	// Go
	"go":   {"go"},
	"mod":  {tagGoMod},
	"sum":  {tagGoSum},
	"tmpl": {"go-template"},

	// Rust
	"rs": {"rust"},

	// Java / JVM
	"java":   {tagJava},
	"class":  {tagJava, TagBinary},
	"jar":    {tagJava, TagBinary},
	"kt":     {tagKotlin},
	"kts":    {tagKotlin},
	"scala":  {"scala"},
	"groovy": {tagGroovy},
	"gradle": {tagGroovy, "gradle"},
	"clj":    {tagClojure},
	"cljs":   {"clojurescript"},
	"cljc":   {tagClojure},
	"edn":    {tagClojure, "edn"},

	// C / C++ / ObjC
	"c":     {tagC},
	"h":     {tagC, tagHeader},
	"cpp":   {tagCPlusPlus},
	"cc":    {tagCPlusPlus},
	"cxx":   {tagCPlusPlus},
	"c++":   {tagCPlusPlus},
	"hpp":   {tagCPlusPlus, tagHeader},
	"hxx":   {tagCPlusPlus, tagHeader},
	"hh":    {tagCPlusPlus, tagHeader},
	"inl":   {tagCPlusPlus},
	"ipp":   {tagCPlusPlus},
	"tcc":   {tagCPlusPlus},
	"cs":    {tagCSharp},
	"csx":   {tagCSharp},
	"swift": {"swift"},
	"m":     {"objective-c"},
	"mm":    {"objective-c++"},

	// Perl
	"pl":  {tagPerl},
	"pm":  {tagPerl},
	"pod": {tagPerl, "pod"},
	"t":   {tagPerl},

	// R
	"r":   {tagR},
	"R":   {tagR},
	"rmd": {tagR, tagMarkdown},
	"Rmd": {tagR, tagMarkdown},
	"rnw": {tagR},
	"Rnw": {tagR},

	// Lua
	"lua":  {tagLua},
	"luau": {tagLua},

	// Shell
	"sh":   {tagShell, tagBash},
	"bash": {tagShell, tagBash},
	"zsh":  {tagShell, tagZsh},
	"fish": {tagShell, tagFish},
	"ksh":  {tagShell, "ksh"},
	"csh":  {tagShell, "csh"},
	"tcsh": {tagShell, "tcsh"},
	"ash":  {tagShell, "ash"},
	"dash": {tagShell},
	"bats": {tagShell, tagBash, "bats"},
	"ps1":  {tagPowerShell},
	"psm1": {tagPowerShell},
	"psd1": {tagPowerShell},
	"bat":  {tagBatch},
	"cmd":  {tagBatch},

	// PHP
	"php":   {tagPHP},
	"phtml": {tagPHP},
	"php3":  {tagPHP},
	"php4":  {tagPHP},
	"php5":  {tagPHP},
	"phps":  {tagPHP},

	// Markup and data formats
	"json":     {tagJSON},
	"jsonl":    {tagJSON},
	"json5":    {"json5"},
	"geojson":  {tagJSON, "geojson"},
	"yaml":     {tagYAML},
	"yml":      {tagYAML},
	"toml":     {tagTOML},
	"xml":      {tagXML},
	"xsl":      {tagXML, tagXSL},
	"xslt":     {tagXML, tagXSL},
	"xsd":      {tagXML, "xsd"},
	"dtd":      {tagXML, "dtd"},
	"plist":    {tagXML, "plist"},
	"html":     {tagHTML},
	"htm":      {tagHTML},
	"xhtml":    {tagHTML, "xhtml"},
	"vue":      {"vue"},
	"svelte":   {"svelte"},
	"css":      {tagCSS},
	"scss":     {"scss", tagCSS},
	"sass":     {"sass", tagCSS},
	"less":     {"less", tagCSS},
	"styl":     {"stylus", tagCSS},
	"md":       {tagMarkdown},
	"mdx":      {tagMarkdown, "mdx"},
	"markdown": {tagMarkdown},
	"rst":      {tagRST},
	"tex":      {tagTeX, tagLaTeX},
	"latex":    {tagTeX, tagLaTeX},
	"bib":      {"bib", tagLaTeX},
	"csv":      {"csv"},
	"tsv":      {"tsv"},

	// Config files
	"ini":        {tagINI},
	"cfg":        {tagINI},
	"conf":       {"conf"},
	"properties": {"properties", "java-properties"},
	"env":        {tagDotenv},
	"envrc":      {tagDotenv},
	"gitconfig":  {"gitconfig"},

	// Images
	"png":  {tagImage, "png"},
	"jpg":  {tagImage, tagJPEG},
	"jpeg": {tagImage, tagJPEG},
	"gif":  {tagImage, "gif"},
	"bmp":  {tagImage, "bmp"},
	"tiff": {tagImage, tagTIFF},
	"tif":  {tagImage, tagTIFF},
	"webp": {tagImage, "webp"},
	"svg":  {tagImage, "svg"},
	"ico":  {tagImage, tagIcon},
	"icns": {tagImage, tagIcon},
	"psd":  {tagImage, "psd"},
	"ai":   {tagImage, "ai"},
	"eps":  {tagImage, "eps"},

	// Audio/Video
	"mp3":  {tagAudio},
	"mp4":  {tagVideo},
	"avi":  {tagVideo},
	"mkv":  {tagVideo},
	"mov":  {tagVideo},
	"wav":  {tagAudio},
	"flac": {tagAudio},
	"ogg":  {tagAudio},
	"webm": {tagVideo},

	// Fonts
	"ttf":   {tagFont},
	"otf":   {tagFont},
	"woff":  {tagFont},
	"woff2": {tagFont},
	"eot":   {tagFont},

	// Documents
	"pdf":  {"pdf"},
	"doc":  {tagDocument},
	"docx": {tagDocument},
	"xls":  {tagDocument},
	"xlsx": {tagDocument},
	"ppt":  {tagDocument},
	"pptx": {tagDocument},
	"odt":  {tagDocument},
	"ods":  {tagDocument},
	"odp":  {tagDocument},
	"rtf":  {tagDocument, "rtf"},

	// Build, package, and DevOps
	"sql":     {"sql"},
	"tf":      {tagTerraform, tagHCL},
	"tfvars":  {tagTerraform, tagHCL},
	"hcl":     {tagHCL},
	"proto":   {"protobuf"},
	"thrift":  {"thrift"},
	"avro":    {tagAvro},
	"graphql": {tagGraphQL},
	"gql":     {tagGraphQL},

	// Haskell
	"hs":    {tagHaskell},
	"lhs":   {tagHaskell},
	"cabal": {tagHaskell, "cabal"},

	// Elixir / Erlang
	"ex":  {tagElixir},
	"exs": {tagElixir},
	"erl": {tagErlang},
	"hrl": {tagErlang},

	// Dart / Flutter
	"dart": {tagDart},

	// Zig
	"zig": {"zig"},

	// Nim
	"nim":    {tagNim},
	"nimble": {tagNim},

	// Julia
	"jl": {"julia"},

	// F# / OCaml / ML
	"fs":  {tagFSharp},
	"fsx": {tagFSharp},
	"fsi": {tagFSharp},
	"ml":  {tagOCaml},
	"mli": {tagOCaml},
	"re":  {tagReason},
	"rei": {tagReason},

	// Lisp family
	"el":   {"emacs-lisp", tagLisp},
	"lisp": {tagLisp},
	"lsp":  {tagLisp},
	"scm":  {"scheme", tagLisp},
	"rkt":  {"racket", tagLisp},
	"hy":   {"hy", tagLisp},

	// Assembly
	"asm":  {tagAssembly},
	"s":    {tagAssembly},
	"S":    {tagAssembly},
	"nasm": {tagAssembly},

	// Fortran
	"f":   {tagFortran},
	"f90": {tagFortran},
	"f95": {tagFortran},
	"f03": {tagFortran},
	"for": {tagFortran},

	// D
	"d":  {tagDlang},
	"di": {tagDlang},

	// V
	"v": {"vlang"},

	// Crystal
	"cr": {"crystal"},

	// Templates
	"j2":       {tagJinja},
	"jinja":    {tagJinja},
	"jinja2":   {tagJinja},
	"twig":     {"twig"},
	"ejs":      {"ejs"},
	"hbs":      {"handlebars"},
	"mustache": {"mustache"},
	"pug":      {tagPug},
	"jade":     {tagPug},
	"slim":     {"slim"},
	"haml":     {"haml"},

	// Config / Schema
	"jsonschema": {tagJSON, "jsonschema"},
	"avsc":       {tagJSON, tagAvro},
	"prisma":     {"prisma"},
	"nginx":      {"nginx"},

	// Notebooks / Data
	"ipynb": {"jupyter"},
	"rdata": {tagR},
	"rds":   {tagR},
	"sas":   {"sas"},
	"sps":   {"spss"},
	"do":    {tagStata},
	"dta":   {tagStata},
	"mat":   {"matlab"},

	// Makefile-like
	"mk":    {tagMakefile},
	"cmake": {tagCMake},

	// Docker / CI
	"dockerignore": {tagDockerignore},

	// Nix
	"nix": {"nix"},

	// Cue
	"cue": {"cue"},

	// Jsonnet
	"jsonnet":   {tagJsonnet},
	"libsonnet": {tagJsonnet},

	// WASM
	"wasm": {"wasm", TagBinary},
	"wat":  {"wasm-text"},

	// Certificates / Keys
	"pem": {tagPEM},
	"crt": {tagPEM, tagCertificate},
	"key": {tagPEM, "key"},
	"cer": {tagCertificate},
	"p12": {tagCertificate, TagBinary},
	"pfx": {tagCertificate, TagBinary},

	// Archives
	"zip": {tagArchive, TagBinary},
	"tar": {tagArchive, TagBinary},
	"gz":  {tagArchive, TagBinary},
	"bz2": {tagArchive, TagBinary},
	"xz":  {tagArchive, TagBinary},
	"7z":  {tagArchive, TagBinary},
	"rar": {tagArchive, TagBinary},
	"tgz": {tagArchive, TagBinary},

	// Misc
	"diff":  {tagDiff},
	"patch": {tagDiff, "patch"},
	"log":   {"log"},
	"lock":  {tagLock},
	"pid":   {"pid"},
	"map":   {"sourcemap"},
	"wsdl":  {tagXML, "wsdl"},
}

// filenameMap maps specific filenames to type tags.
var filenameMap = map[string][]string{
	"makefile":       {tagMakefile},
	"gnumakefile":    {tagMakefile},
	"dockerfile":     {tagDockerfile},
	"containerfile":  {tagDockerfile},
	"vagrantfile":    {tagRuby},
	"gemfile":        {tagRuby},
	"gemfile.lock":   {tagRuby, tagLock},
	"rakefile":       {tagRuby},
	"cmakelists.txt": {tagCMake},
	"jenkinsfile":    {tagGroovy},
	"procfile":       {"procfile"},
	"brewfile":       {tagRuby},

	// Git
	".gitignore":     {"gitignore"},
	".gitattributes": {"gitattributes"},
	".gitmodules":    {"gitmodules"},
	".gitkeep":       {"gitkeep"},
	".mailmap":       {"mailmap"},

	// Editor
	".editorconfig":   {"editorconfig"},
	".prettierrc":     {tagJSON, tagPrettier},
	".prettierignore": {tagPrettier},
	".eslintrc":       {tagJSON, tagESLint},
	".eslintrc.json":  {tagJSON, tagESLint},
	".eslintrc.yml":   {tagYAML, tagESLint},
	".eslintrc.yaml":  {tagYAML, tagESLint},
	".eslintrc.js":    {tagJavaScript, tagESLint},
	".eslintignore":   {tagESLint},
	".stylelintrc":    {tagJSON},
	".babelrc":        {tagJSON},
	".browserslistrc": {"browserslist"},

	// Package managers
	"package.json":      {tagJSON, tagPackageJSON},
	"package-lock.json": {tagJSON, tagPackageJSON, tagLock},
	"tsconfig.json":     {tagJSON, "tsconfig"},
	"yarn.lock":         {tagYAML, tagLock},
	"pnpm-lock.yaml":    {tagYAML, tagLock},
	"composer.json":     {tagJSON, "composer"},
	"composer.lock":     {tagJSON, tagLock},
	"cargo.toml":        {tagTOML, "cargo"},
	"cargo.lock":        {tagTOML, tagLock},
	"go.mod":            {tagGoMod},
	"go.sum":            {tagGoSum},
	"pubspec.yaml":      {tagYAML, tagDart},
	"pubspec.lock":      {tagYAML, tagDart, tagLock},
	"requirements.txt":  {"requirements-txt", tagPip},
	"setup.py":          {tagPython, tagSetuptools},
	"setup.cfg":         {tagINI, tagSetuptools},
	// Upstream identify tags this {toml, pyproject} — NOT python. Tagging it
	// python makes python-only hooks (black, isort, …) try to parse TOML.
	"pyproject.toml": {tagTOML, "pyproject"},
	"pipfile":        {tagTOML, tagPip},
	"pipfile.lock":   {tagJSON, tagPip, tagLock},
	"poetry.lock":    {tagTOML, tagLock},

	// CI
	".travis.yml":            {tagYAML, "travis"},
	".circleci/config.yml":   {tagYAML, "circleci"},
	"appveyor.yml":           {tagYAML, "appveyor"},
	"azure-pipelines.yml":    {tagYAML, "azure"},
	".github/dependabot.yml": {tagYAML, "dependabot"},
	"netlify.toml":           {tagTOML, "netlify"},
	"vercel.json":            {tagJSON, "vercel"},
	"render.yaml":            {tagYAML},

	// Docker
	"docker-compose.yml":  {tagYAML, tagDockerCompose},
	"docker-compose.yaml": {tagYAML, tagDockerCompose},
	".dockerignore":       {tagDockerignore},

	// Config files
	".pre-commit-config.yaml": {tagYAML, tagPreCommit},
	".pre-commit-hooks.yaml":  {tagYAML, tagPreCommit},
	".flake8":                 {tagINI, "flake8"},
	".pylintrc":               {tagINI, "pylint"},
	".rubocop.yml":            {tagYAML, "rubocop"},
	".yamllint":               {tagYAML},
	".yamllint.yml":           {tagYAML},
	".yamllint.yaml":          {tagYAML},
	".markdownlint.yml":       {tagYAML},
	".clang-format":           {tagYAML, "clang-format"},
	".clang-tidy":             {tagYAML},
	"tox.ini":                 {tagINI, "tox"},
	"mypy.ini":                {tagINI, tagMypy},
	".mypy.ini":               {tagINI, tagMypy},
	".coveragerc":             {tagINI, "coverage"},
	".isort.cfg":              {tagINI, "isort"},
	"rustfmt.toml":            {tagTOML, tagRustfmt},
	".rustfmt.toml":           {tagTOML, tagRustfmt},
	"clippy.toml":             {tagTOML, tagClippy},
	".clippy.toml":            {tagTOML, tagClippy},
	".golangci.yml":           {tagYAML, tagGolangci},
	".golangci.yaml":          {tagYAML, tagGolangci},
	".goreleaser.yml":         {tagYAML, tagGoReleaser},
	".goreleaser.yaml":        {tagYAML, tagGoReleaser},

	// Shell
	".bashrc":       {tagShell, tagBash},
	".bash_profile": {tagShell, tagBash},
	".bash_logout":  {tagShell, tagBash},
	".zshrc":        {tagShell, tagZsh},
	".zshenv":       {tagShell, tagZsh},
	".zprofile":     {tagShell, tagZsh},
	".profile":      {tagShell},
	".inputrc":      {"inputrc"},

	// Env
	".env":             {tagDotenv},
	".env.example":     {tagDotenv},
	".env.local":       {tagDotenv},
	".env.development": {tagDotenv},
	".env.production":  {tagDotenv},
	".env.test":        {tagDotenv},
	".envrc":           {tagDotenv, "direnv"},

	// License
	"license":     {tagLicense},
	"licence":     {tagLicense}, //nolint:misspell // British English filename
	"license.md":  {tagLicense, tagMarkdown},
	"licence.md":  {tagLicense, tagMarkdown},
	"license.txt": {tagLicense},
	"licence.txt": {tagLicense},
	"copying":     {tagLicense},

	// Docs
	"readme":       {tagReadme},
	"readme.md":    {tagReadme, tagMarkdown},
	"readme.rst":   {tagReadme, tagRST},
	"readme.txt":   {tagReadme},
	"changelog":    {tagChangelog},
	"changelog.md": {tagChangelog, tagMarkdown},
	"changes":      {tagChangelog},
	"authors":      {tagAuthors},
	"contributors": {tagAuthors},
	"codeowners":   {tagCodeowners},
	".codeowners":  {tagCodeowners},
}

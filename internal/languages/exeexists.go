// SPDX-FileCopyrightText: 2026 Blair Hamilton
// SPDX-License-Identifier: Apache-2.0

package languages

import (
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// shimsRE is upstream lang_base.SHIMS_RE: an executable found in a shims
// directory (asdf, pyenv, rbenv, ...) is a version manager's trampoline, not
// a real install.
var shimsRE = regexp.MustCompile(`[/\\]shims[/\\]`)

// exeExists is upstream lang_base.exe_exists (4.7.0): the executable is on
// PATH, is not a version manager's shim, and is not inside the home directory
// unless the home directory is the filesystem root (a container or service
// user). Languages use it to decide whether the host's toolchain is a "system"
// install worth reusing or whether to provision their own.
func exeExists(exe string) bool {
	found, ok := pyFindExecutable(exe, os.Getenv("PATH"), os.Getenv("PATHEXT"), pathextSet())
	if !ok {
		return false
	}
	return exeIsSystem(runtime.GOOS, found, pyHomeDir())
}

// exeIsSystem is the decision half of exe_exists, for an executable found at
// found and a home directory home, with goos's path rules: not under a shims
// directory, and either home is the filesystem root or found is not inside
// it (which includes commonpath raising, as it does across Windows drives).
func exeIsSystem(goos, found, home string) bool {
	common, ok := pyCommonPathFor(goos, found, home)
	return !shimsRE.MatchString(found) &&
		(pyDirnameFor(goos, home) == home || !ok || common != home)
}

// pathextSet reports whether PATHEXT is set at all, which is what upstream's
// find_executable keys on ('PATHEXT' in environ), not whether it is empty.
func pathextSet() bool {
	_, ok := os.LookupEnv("PATHEXT")
	return ok
}

// pyFindExecutable is upstream parse_shebang.find_executable: a name with a
// separator is returned as is; otherwise each PATH entry is tried, with each
// PATHEXT extension and then the bare name, and the first regular file that
// is executable wins.
func pyFindExecutable(exe, path, pathext string, hasPathext bool) (string, bool) {
	exe = filepath.Clean(exe)
	if strings.ContainsRune(exe, filepath.Separator) {
		return exe, true
	}
	names := []string{exe}
	if hasPathext {
		names = nil
		for _, ext := range strings.Split(pathext, string(os.PathListSeparator)) {
			names = append(names, exe+ext)
		}
		names = append(names, exe)
	}
	for _, dir := range strings.Split(path, string(os.PathListSeparator)) {
		for _, name := range names {
			joined := name
			if dir != "" {
				joined = filepath.Join(dir, name)
			}
			info, err := os.Stat(joined)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
				continue
			}
			return joined, true
		}
	}
	return "", false
}

// pyHomeDir is os.path.expanduser('~'): HOME (USERPROFILE on Windows) when
// set, even if empty, else the account's home directory.
func pyHomeDir() string {
	key := "HOME"
	if runtime.GOOS == "windows" {
		key = "USERPROFILE"
	}
	h, ok := os.LookupEnv(key)
	if !ok {
		u, err := user.Current()
		if err != nil {
			return "~"
		}
		h = u.HomeDir
	}
	if runtime.GOOS == "windows" {
		return h
	}
	// posixpath.expanduser strips trailing slashes, and an empty result
	// (HOME unset to "" or "/") becomes the root.
	if h = strings.TrimRight(h, "/"); h == "" {
		return "/"
	}
	return h
}

// pyDirname is os.path.dirname for this platform.
func pyDirname(p string) string { return pyDirnameFor(runtime.GOOS, p) }

// pyDirnameFor is posixpath.dirname, or ntpath.dirname when goos is windows:
// everything before the last separator, with trailing separators removed
// unless that leaves nothing but the root. A Windows drive (C: or a UNC
// \\server\share) and its root are kept whole, so dirname(`C:\`) is `C:\`.
func pyDirnameFor(goos, p string) string {
	drive, rest := splitDriveFor(goos, p)
	seps := pathSepsFor(goos)
	i := strings.LastIndexAny(rest, seps)
	if i < 0 {
		return drive
	}
	head := rest[:i+1]
	if trimmed := strings.TrimRight(head, seps); trimmed != "" {
		return drive + trimmed
	}
	return drive + head
}

func pathSepsFor(goos string) string {
	if goos == "windows" {
		return `\/`
	}
	return "/"
}

// splitDriveFor is ntpath.splitdrive on windows (a drive letter and colon,
// or the \\server\share of a UNC path) and posixpath's, which has no drive.
func splitDriveFor(goos, p string) (drive, rest string) {
	if goos != "windows" {
		return "", p
	}
	isSep := func(c byte) bool { return c == '\\' || c == '/' }
	if len(p) >= 2 && p[1] == ':' {
		return p[:2], p[2:]
	}
	if len(p) >= 2 && isSep(p[0]) && isSep(p[1]) && (len(p) == 2 || !isSep(p[2])) {
		// \\server\share: the drive runs to the separator after share.
		i := strings.IndexAny(p[2:], `\/`)
		if i < 0 {
			return p, ""
		}
		i += 2
		j := strings.IndexAny(p[i+1:], `\/`)
		if j < 0 {
			return p, ""
		}
		return p[:i+1+j], p[i+1+j:]
	}
	return "", p
}

// pyCommonPath is os.path.commonpath of two paths for this platform.
func pyCommonPath(a, b string) (string, bool) { return pyCommonPathFor(runtime.GOOS, a, b) }

// pyCommonPathFor is posixpath.commonpath, or ntpath.commonpath when goos is
// windows, of two paths; ok is false where Python raises ValueError (one path
// absolute and the other relative, or, on Windows, different drives). On
// Windows the comparison ignores case but the result is spelled as in a, as
// ntpath's is.
func pyCommonPathFor(goos, a, b string) (string, bool) {
	volA, a := splitDriveFor(goos, a)
	volB, b := splitDriveFor(goos, b)
	if !strings.EqualFold(volA, volB) {
		return "", false
	}
	seps := pathSepsFor(goos)
	isSep := func(r rune) bool { return strings.ContainsRune(seps, r) }
	absA := a != "" && isSep(rune(a[0]))
	absB := b != "" && isSep(rune(b[0]))
	if absA != absB {
		return "", false
	}
	parts := func(p string) []string {
		var out []string
		for _, s := range strings.FieldsFunc(p, isSep) {
			if s != "." {
				out = append(out, s)
			}
		}
		return out
	}
	pa, pb := parts(a), parts(b)
	n := 0
	for n < len(pa) && n < len(pb) {
		if goos == "windows" {
			if !strings.EqualFold(pa[n], pb[n]) {
				break
			}
		} else if pa[n] != pb[n] {
			break
		}
		n++
	}
	sep := "/"
	if goos == "windows" {
		sep = `\`
	}
	prefix := volA
	if absA {
		prefix += sep
	}
	return prefix + strings.Join(pa[:n], sep), true
}

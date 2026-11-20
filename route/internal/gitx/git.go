// Package gitx wraps the git plumbing the backend needs: reading a commit
// (including its gpgsig armor), diffstat, graph digest, signature
// verification, and the staging/commit verbs behind the CRUD commands.
package gitx

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Commit is the subset of a git commit the envelope needs.
type Commit struct {
	Sha         string
	Tree        string
	Parents     []string
	Headers     []string // tree/parent/author/committer lines, in order, without gpgsig
	AuthorName  string
	AuthorEmail string
	CommittedAt time.Time
	Message     string
	Sig         string
	Signed      bool
}

// Payload reconstructs the bytes the signature covers: the header block
// without the gpgsig header, a blank line, then the message.
func (c *Commit) Payload() []byte {
	body := strings.Join(c.Headers, "\n") + "\n\n" + c.Message + "\n"
	return []byte(body)
}

var identRE = regexp.MustCompile(`^(.+) <([^>]+)> (\d+) ([+-]\d{4})$`)

// Run executes git in dir and returns stdout, or an error carrying stderr.
func Run(dir string, args ...string) (string, error) {
	out, errs, err := run(dir, args...)
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errs))
	}
	return out, nil
}

func run(dir string, args ...string) (string, string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	return out.String(), errb.String(), err
}

// Head resolves a revision to a full sha.
func Head(dir, rev string) (string, error) {
	if rev == "" {
		rev = "HEAD"
	}
	out, err := Run(dir, "rev-parse", rev)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ReadCommit parses `git cat-file -p <sha>`, including the multi-line gpgsig
// header whose continuation lines start with a single space.
func ReadCommit(dir, sha string) (*Commit, error) {
	raw, err := Run(dir, "cat-file", "-p", sha)
	if err != nil {
		return nil, err
	}
	c := &Commit{Sha: sha}
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	inSig, inMsg := false, false
	for sc.Scan() {
		line := sc.Text()
		if inSig {
			if strings.HasPrefix(line, " ") {
				c.Sig += strings.TrimPrefix(line, " ") + "\n"
				continue
			}
			inSig = false
		}
		if inMsg {
			c.Message += line + "\n"
			continue
		}
		switch {
		case strings.HasPrefix(line, "tree "):
			c.Tree = strings.TrimPrefix(line, "tree ")
			c.Headers = append(c.Headers, line)
		case strings.HasPrefix(line, "parent "):
			c.Parents = append(c.Parents, strings.TrimPrefix(line, "parent "))
			c.Headers = append(c.Headers, line)
		case strings.HasPrefix(line, "author "):
			if m := identRE.FindStringSubmatch(strings.TrimPrefix(line, "author ")); m != nil {
				c.AuthorName, c.AuthorEmail = m[1], m[2]
			}
			c.Headers = append(c.Headers, line)
		case strings.HasPrefix(line, "committer "):
			if m := identRE.FindStringSubmatch(strings.TrimPrefix(line, "committer ")); m != nil {
				if ts, err := strconv.ParseInt(m[3], 10, 64); err == nil {
					c.CommittedAt = time.Unix(ts, 0).UTC()
				}
			}
			c.Headers = append(c.Headers, line)
		case strings.HasPrefix(line, "gpgsig "):
			c.Sig = strings.TrimPrefix(line, "gpgsig ") + "\n"
			c.Signed = true
			inSig = true
		case line == "":
			inMsg = true
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if c.CommittedAt.IsZero() {
		c.CommittedAt = time.Unix(0, 0).UTC()
	}
	c.Message = strings.TrimRight(c.Message, "\n")
	return c, nil
}

// DiffStat returns the file/insertion/deletion counts of a commit.
func DiffStat(dir, sha string) (files, insertions, deletions int, err error) {
	out, err := Run(dir, "show", "--shortstat", "--format=", sha)
	if err != nil {
		return 0, 0, 0, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, 0, 0, nil
	}
	num := func(re string) int {
		m := regexp.MustCompile(re).FindStringSubmatch(out)
		if m == nil {
			return 0
		}
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return num(`(\d+) files? changed`), num(`(\d+) insertions?\(\+\)`), num(`(\d+) deletions?\(-\)`), nil
}

// GraphDigest is the sha256 of `git log --graph --oneline -n 20`, so the
// envelope binds to the shape of history and not just to one commit.
func GraphDigest(dir string) (string, error) {
	out, err := Run(dir, "log", "--graph", "--oneline", "-n", "20")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(out))
	return hex.EncodeToString(sum[:]), nil
}

// VerifyCommit checks the commit signature with git itself and returns the
// signer fingerprint. Unsigned or bad signatures yield ok=false and no error.
// Verification always uses the local gpg binary, never the Codespaces helper.
func VerifyCommit(dir, sha string) (fpr string, ok bool, err error) {
	args := []string{"verify-commit", "--raw", sha}
	if program := gpgProgram(); program != "" {
		args = append([]string{"-c", "gpg.program=" + program}, args...)
	}
	out, errs, runErr := run(dir, args...)
	combined := out + "\n" + errs
	for _, line := range strings.Split(combined, "\n") {
		fields := strings.Fields(line)
		for i, tok := range fields {
			if tok == "VALIDSIG" && i+1 < len(fields) {
				fpr = fields[i+1]
			}
		}
	}
	if runErr != nil || fpr == "" {
		return fpr, false, nil
	}
	return fpr, true, nil
}

// AddIntent records the path with `git add -N` (there is no `git add -b`).
func AddIntent(dir string, paths ...string) error {
	args := append([]string{"add", "-N", "--"}, paths...)
	_, err := Run(dir, args...)
	return err
}

// Add stages the content of paths.
func Add(dir string, paths ...string) error {
	args := append([]string{"add", "--"}, paths...)
	_, err := Run(dir, args...)
	return err
}

// gpgProgram resolves the real GnuPG binary. Codespaces sets
// gpg.program=/.codespaces/bin/gh-gpgsign in /etc/gitconfig; that helper
// signs through GitHub and returns 403 when repo signing is not enabled, so
// verification always uses the local binary.
func gpgProgram() string {
	path, err := exec.LookPath("gpg")
	if err != nil {
		return ""
	}
	return path
}

// configuredSignProgram returns the real gpg when the configured program is
// the Codespaces signing helper, and "" otherwise (respect the user's setup).
func configuredSignProgram(dir string) string {
	out, err := Run(dir, "config", "--get", "gpg.program")
	if err != nil {
		return ""
	}
	if strings.Contains(strings.TrimSpace(out), "gh-gpgsign") {
		return gpgProgram()
	}
	return ""
}

// CommitStaged commits the index. sign=true passes -S so the commit carries
// a gpgsig block produced by the signing key; program overrides gpg.program
// for that invocation ("" respects the repository configuration).
func CommitStaged(dir, message string, sign bool, program string) (string, error) {
	args := []string{"commit", "-m", message}
	if sign {
		args = []string{"commit", "-S", "-m", message}
		if program == "" {
			program = configuredSignProgram(dir)
		}
		if program != "" {
			args = append([]string{"-c", "gpg.program=" + program}, args...)
		}
	}
	if _, err := Run(dir, args...); err != nil {
		return "", err
	}
	return Head(dir, "HEAD")
}

// Amend rewrites the last commit, optionally signing it.
func Amend(dir, message string, sign bool, program string) (string, error) {
	args := []string{"commit", "--amend", "--no-edit"}
	if message != "" {
		args = []string{"commit", "--amend", "-m", message}
	}
	if sign {
		args = append(args, "-S")
		if program == "" {
			program = configuredSignProgram(dir)
		}
		if program != "" {
			args = append([]string{"-c", "gpg.program=" + program}, args...)
		}
	}
	if _, err := Run(dir, args...); err != nil {
		return "", err
	}
	return Head(dir, "HEAD")
}

// Init creates a repository (used by tests and by `route init`).
func Init(dir string) error {
	_, err := Run(dir, "init", "-q")
	return err
}

// Config sets a repository-local config value.
func Config(dir, key, value string) error {
	_, err := Run(dir, "config", key, value)
	return err
}

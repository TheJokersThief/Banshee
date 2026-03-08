package gitcli

import "errors"

var (
	ErrBranchAlreadyExists = errors.New("git: branch already exists")
	ErrAlreadyUpToDate     = errors.New("git: already up to date")
	ErrReferenceNotFound   = errors.New("git: reference not found")
	ErrRemoteRefNotFound   = errors.New("git: couldn't find remote ref")
)

type Git interface {
	Clone(tokenURL, dir, branch string, depth int) error
	Checkout(dir, branch string, create bool) error
	Fetch(dir, tokenURL, branch string) error
	Pull(dir, tokenURL, branch string) error
	Push(dir, tokenURL, branch string) error
	IsClean(dir string) (bool, error)
	AddAll(dir string) error
	Commit(dir, message, authorName, authorEmail string) error
}

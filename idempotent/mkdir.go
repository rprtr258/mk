package idempotent

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"github.com/rprtr258/fun"
)

const DefaultDirPerm = 0o644

type mkdirInspection struct {
	exists      bool
	isDir       bool
	arePermSame bool
}

// TODO: steal config, semantics from ansible
type mkdir struct {
	dirname    string
	perm       fs.FileMode
	inspection *mkdirInspection
}

type MkdirOptions struct {
	// Dirname to create
	Dirname string
	// Perm for dirname to use. DefaultDirPerm is used by default.
	Perm fs.FileMode
}

func NewMkdir(opts MkdirOptions) Action {
	perm := fun.If(opts.Perm != 0, opts.Perm, DefaultDirPerm)

	return &mkdir{
		dirname:    opts.Dirname,
		perm:       perm,
		inspection: nil,
	}
}

func (a *mkdir) inspect() error {
	if a.inspection != nil {
		return nil
	}

	info, err := os.Stat(a.dirname)
	if err != nil {
		if os.IsNotExist(err) {
			a.inspection = &mkdirInspection{
				exists:      false,
				isDir:       false,
				arePermSame: false,
			}
		}

		return fmt.Errorf("stat %q: %w", a.dirname, err)
	}

	a.inspection = &mkdirInspection{
		exists:      true,
		isDir:       info.IsDir(),
		arePermSame: info.Mode().Perm() == a.perm,
	}

	return nil
}

func (a *mkdir) IsCompleted() (bool, error) {
	if err := a.inspect(); err != nil {
		return false, err
	}

	return a.inspection.exists &&
		a.inspection.isDir &&
		a.inspection.arePermSame, nil
}

func (a *mkdir) Run(ctx context.Context) error {
	if err := a.inspect(); err != nil {
		return err
	}

	if !a.inspection.exists {
		if err := os.MkdirAll(a.dirname, a.perm); err != nil {
			return fmt.Errorf("mkdir dirname=%q mode=%v: %w", a.dirname, a.perm, err)
		}
		return nil
	}

	if !a.inspection.isDir {
		return fmt.Errorf("%q is a file, not directory", a.dirname)
	}

	if err := os.Chmod(a.dirname, a.perm); err != nil {
		return fmt.Errorf("chmod dirname=%q mode=%v: %w", a.dirname, a.perm, err)
	}

	return nil
}

package source

import (
	"errors"
	"io/fs"
	"slices"
)

type filteredFS struct {
	fsys   fs.FS
	ignore []string
}

func newFilteredFS(fsys fs.FS, ignore []string) filteredFS {
	return filteredFS{fsys: fsys, ignore: ignore}
}

func (f filteredFS) Open(name string) (fs.File, error) {
	file, err := f.fsys.Open(name)
	if err != nil {
		return nil, err
	}
	return &filteredFile{File: file, ignore: f.ignore}, nil
}

func (f filteredFS) ReadLink(name string) (string, error) {
	rl, ok := f.fsys.(fs.ReadLinkFS)
	if !ok {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: errors.ErrUnsupported}
	}
	return rl.ReadLink(name)
}

type filteredFile struct {
	fs.File
	ignore []string
}

func (f *filteredFile) ReadDir(n int) (entries []fs.DirEntry, err error) {
	dir, ok := f.File.(fs.ReadDirFile)
	if !ok {
		return nil, &fs.PathError{Op: "readdir", Path: ".", Err: errors.New("not a directory")}
	}
	all, err := dir.ReadDir(n)
	if err != nil {
		return all, err
	}
	entries = make([]fs.DirEntry, 0, len(all))
	for _, e := range all {
		if !slices.Contains(f.ignore, e.Name()) {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

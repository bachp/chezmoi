package chezmoi

import (
	"archive/tar"
	"os"
	"path/filepath"

	vfs "github.com/twpayne/go-vfs"
)

// A Dir represents the target state of a directory.
type Dir struct {
	sourceName string
	targetName string
	Perm       os.FileMode
	Entries    map[string]Entry
}

type dirConcreteValue struct {
	Type       string        `json:"type" yaml:"type"`
	SourcePath string        `json:"sourcePath" yaml:"sourcePath"`
	TargetPath string        `json:"targetPath" yaml:"targetPath"`
	Perm       int           `json:"perm" yaml:"perm"`
	Entries    []interface{} `json:"entries" yaml:"entries"`
}

// newDir returns a new directory state.
func newDir(sourceName string, targetName string, perm os.FileMode) *Dir {
	return &Dir{
		sourceName: sourceName,
		targetName: targetName,
		Perm:       perm,
		Entries:    make(map[string]Entry),
	}
}

// Apply ensures that targetDir in fs matches d.
func (d *Dir) Apply(fs vfs.FS, targetDir string, umask os.FileMode, mutator Mutator) error {
	targetPath := filepath.Join(targetDir, d.targetName)
	info, err := fs.Lstat(targetPath)
	switch {
	case err == nil && info.Mode().IsDir():
		if info.Mode()&os.ModePerm != d.Perm&^umask {
			if err := mutator.Chmod(targetPath, d.Perm&^umask); err != nil {
				return err
			}
		}
	case err == nil:
		if err := mutator.RemoveAll(targetPath); err != nil {
			return err
		}
		fallthrough
	case os.IsNotExist(err):
		if err := mutator.Mkdir(targetPath, d.Perm&^umask); err != nil {
			return err
		}
	default:
		return err
	}
	for _, entryName := range sortedEntryNames(d.Entries) {
		if err := d.Entries[entryName].Apply(fs, targetDir, umask, mutator); err != nil {
			return err
		}
	}
	return nil
}

// ConcreteValue implements Entry.ConcreteValue.
func (d *Dir) ConcreteValue(targetDir, sourceDir string, recursive bool) (interface{}, error) {
	var entryConcreteValues []interface{}
	if recursive {
		for _, entryName := range sortedEntryNames(d.Entries) {
			entryConcreteValue, err := d.Entries[entryName].ConcreteValue(targetDir, sourceDir, recursive)
			if err != nil {
				return nil, err
			}
			entryConcreteValues = append(entryConcreteValues, entryConcreteValue)
		}
	}
	return &dirConcreteValue{
		Type:       "dir",
		SourcePath: filepath.Join(sourceDir, d.SourceName()),
		TargetPath: filepath.Join(targetDir, d.TargetName()),
		Perm:       int(d.Perm),
		Entries:    entryConcreteValues,
	}, nil
}

// Evaluate evaluates all entries in d.
func (d *Dir) Evaluate() error {
	for _, entryName := range sortedEntryNames(d.Entries) {
		if err := d.Entries[entryName].Evaluate(); err != nil {
			return err
		}
	}
	return nil
}

// Private returns true if d is private.
func (d *Dir) Private() bool {
	return d.Perm&os.ModePerm&077 == 0
}

// SourceName implements Entry.SourceName.
func (d *Dir) SourceName() string {
	return d.sourceName
}

// TargetName implements Entry.TargetName.
func (d *Dir) TargetName() string {
	return d.targetName
}

// archive writes d to w.
func (d *Dir) archive(w *tar.Writer, headerTemplate *tar.Header, umask os.FileMode) error {
	header := *headerTemplate
	header.Typeflag = tar.TypeDir
	header.Name = d.targetName
	header.Mode = int64(d.Perm & os.ModePerm &^ umask)
	if err := w.WriteHeader(&header); err != nil {
		return err
	}
	for _, entryName := range sortedEntryNames(d.Entries) {
		if err := d.Entries[entryName].archive(w, headerTemplate, umask); err != nil {
			return err
		}
	}
	return nil
}
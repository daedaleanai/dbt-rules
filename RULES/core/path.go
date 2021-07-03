package core

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"
)

// Path represents an on-disk path that is either an input to or an output from a BuildStep (or both).
type Path interface {
	Absolute() string
	Relative() string
	String() string

	relative() string
}

// inPath is a path relative to the workspace source directory.
type inPath struct {
	rel string
}

// Absolute returns the absolute path.
func (p inPath) Absolute() string {
	return path.Join(sourceDir(), p.rel)
}

// Relative returns the path relative to the workspace source directory.
func (p inPath) Relative() string {
	return p.rel
}

// String representation of an inPath is its quoted absolute path.
func (p inPath) String() string {
	return p.Absolute()
}

func (p inPath) relative() string {
	return p.rel
}

// OutPath is a path relative to the workspace build directory.
type OutPath interface {
	Path
	BuildOutput

	WithExt(ext string) OutPath
	WithFilename(filename string) OutPath
	WithSuffix(suffix string) OutPath

	isOutPath()
}

type outPath struct {
	hash string
	rel  string
}

// Absolute returns the absolute path.
func (p outPath) Absolute() string {
	return path.Join(buildDir(), p.Relative())
}

// Relative returns the path relative to the workspace build directory.
func (p outPath) Relative() string {
	return path.Join(p.hash, p.relative())
}

// String representation of an OutPath is its quoted absolute path.
func (p outPath) String() string {
	return p.Absolute()
}

func (p outPath) relative() string {
	return p.rel
}

// isOutPath makes sure that inPath or Path cannot be used as OutPath.
func (p outPath) isOutPath() {}

func (p outPath) Output() OutPath {
	return p
}

func (p outPath) Outputs() []OutPath {
	return []OutPath{p}
}

// GlobalPath is a global path.
type GlobalPath interface {
	Absolute() string
}

type globalPath struct {
	abs string
}

// Absolute returns absolute path.
func (p globalPath) Absolute() string {
	return p.abs
}

// String representation of a globalPath is its quoted absolute path.
func (p globalPath) String() string {
	return p.Absolute()
}

// WithExt creates an OutPath with the same relative path and the given extension.
func (p outPath) WithExt(ext string) OutPath {
	oldExt := path.Ext(p.rel)
	newRel := fmt.Sprintf("%s.%s", strings.TrimSuffix(p.rel, oldExt), ext)
	return outPath{p.hash, newRel}
}

// WithFilename creates an OutPath with the same relative path and the given filename.
func (p outPath) WithFilename(filename string) OutPath {
	return outPath{p.hash, path.Join(path.Dir(p.rel), filename)}
}

// WithSuffix creates an OutPath with the same relative path and the given suffix.
func (p outPath) WithSuffix(suffix string) OutPath {
	return outPath{p.hash, p.rel + suffix}
}

// NewInPath creates an inPath for a path relativ to the source directory.
func NewInPath(pkg interface{}, p string) Path {
	return inPath{path.Join(reflect.TypeOf(pkg).PkgPath(), p)}
}

// NewOutPath creates an OutPath for a path relativ to the build directory.
func NewOutPath(pkg interface{}, p string) OutPath {
	fmt.Println("NewOutPath should never be called")
	os.Exit(1)
	return outPath{"", ""}
}

// NewGlobalPath creates a globalPath.
func NewGlobalPath(p string) GlobalPath {
	return globalPath{p}
}

// SourcePath returns a path relative to the source directory.
func SourcePath(p string) Path {
	return inPath{p}
}

package devtool

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type FlatMetadata struct {
	path     string
	releases ReleaseMap
	crowbar  *Crowbar
}

type FlatRelease struct {
	name, parent string
	meta         *FlatMetadata
	builds       BuildMap
}

// How to find the pointer to where the on-disk metadata for this release lives.
func (r *FlatRelease) path() string {
	return filepath.Join(r.meta.path, r.name)
}

// Fetch the name of a release
func (r *FlatRelease) Name() string {
	return r.name
}

// Get a map of builds for a specific release.
// They will be indexed in the returned BuildMap y build.Name()
func (r *FlatRelease) Builds() BuildMap {
	if r.builds == nil {
		log.Panicf("Release %s has no builds.", r.name)
	}
	return r.builds
}

// Find the parent release of this release.
// If there isn't one, return nil.
func (r *FlatRelease) Parent() Release {
	if r.parent == "" {
		return nil
	}
	if r.meta.releases[r.parent] == nil {
		log.Panicf("Parent release %s of %s does not exist!", r.parent, r.name)
	}
	return r.meta.releases[r.parent]
}

type FlatBuild struct {
	name, parent string
	release      *FlatRelease
	barclamps    BarclampMap
}

func (b *FlatBuild) path() string {
	return filepath.Join(b.release.path(), b.name)
}

// The basic name of a build.
func (b *FlatBuild) Name() string {
	return b.name
}

// The full name of a build.
// Equal to release.Name() + / + build.Name()
func (b *FlatBuild) FullName() string {
	return b.release.Name() + "/" + b.name
}

// The release that this build is a part of.
func (b *FlatBuild) Release() Release {
	if b.release == nil {
		log.Panicf("Build %s is not a member of any release!", b.name)
	}
	return b.release
}

// The parent build of this one.
func (b *FlatBuild) Parent() Build {
	if b.parent == "" {
		return nil
	}
	if b.release.builds[b.parent] == nil {
		log.Panicf("Release %s: cannot find parent build %s of build %s!",
			b.release.name, b.parent, b.name)
	}
	return b.release.builds[b.parent]
}

// The barclamps that are a part of this build.
func (b *FlatBuild) Barclamps() BarclampMap {
	return b.barclamps
}

// Get a list of releases that this metadata knows about
func (m *FlatMetadata) Releases() ReleaseMap {
	if m.releases == nil {
		log.Panicf("No releases available")
	}
	return m.releases
}

// Get a list of all the builds this metadata knows about.
// The returned BuildMap will have build.FullName() keys.
func (m *FlatMetadata) AllBuilds() BuildMap {
	res := make(BuildMap)
	for _, rel := range m.Releases() {
		for _, bld := range rel.Builds() {
			res[bld.FullName()] = bld
		}
	}
	return res
}

func (m *FlatMetadata) populateBuild(release *FlatRelease, name string) Build {
	build := &FlatBuild{
		name:      name,
		release:   release,
		barclamps: make(BarclampMap),
	}
	bld := build.path()
	parentLink := filepath.Join(bld, "parent")
	stat, err := os.Lstat(parentLink)
	if err == nil && (stat.Mode()&os.ModeSymlink) != 0 {
		parent, err := os.Readlink(parentLink)
		if err == nil {
			build.parent = filepath.Base(parent)
		}
	}
	glob := filepath.Join(bld, "barclamp-*")
	barclamps, err := filepath.Glob(glob)
	c := release.meta.crowbar
	dieIfError(err)
	for _, bc := range barclamps {
		barclamp := &Barclamp{}
		barclamp.Name = strings.TrimPrefix(bc, filepath.Join(bld, "barclamp-"))
		if c.Barclamps[barclamp.Name] == nil {
			log.Panicf("Build %s/%s wants %s, which is not in %s\n",
				release.name,
				build.name,
				barclamp.Name,
				filepath.Join(c.Repo.Path(),
					"barclamps",
					barclamp.Name))
		}
		barclamp.Repo = c.Barclamps[barclamp.Name]
		branch, err := ioutil.ReadFile(bc)
		if err != nil {
			continue
		}
		buf := bytes.NewBuffer(branch)
		barclamp.Branch = strings.TrimSpace(buf.String())
		build.barclamps[barclamp.Name] = barclamp
	}
	return build
}

func (m *FlatMetadata) populateRelease(rel string) *FlatRelease {
	release := &FlatRelease{
		meta:   m,
		name:   rel,
		builds: make(BuildMap),
	}
	prefix := release.path()
	glob := filepath.Join(prefix, "*/")
	parentFile := filepath.Join(prefix, "parent")
	stat, err := os.Stat(parentFile)
	if err == nil && stat.Mode().IsRegular() {
		parent, err := ioutil.ReadFile(parentFile)
		if err == nil {
			buf := bytes.NewBuffer(parent)
			release.parent = strings.TrimSpace(buf.String())
		}
	}
	builds, err := filepath.Glob(glob)
	dieIfError(err)
	for _, bld := range builds {
		bld = strings.Trim(strings.TrimPrefix(bld, prefix), "/")
		build := m.populateBuild(release, bld)
		dieIfError(err)
		release.builds[bld] = build
	}
	return release
}

// Populate the Releases field of a Crowbar struct, if we are using flat metadata.
func (m *FlatMetadata) Probe(c *Crowbar) (err error) {
	m.path = filepath.Join(c.Repo.Path(), "releases")
	m.crowbar = c
	m.releases = make(ReleaseMap)
	stat, err := os.Lstat(m.path)
	if err != nil {
		return fmt.Errorf("Cannot find %s, metadata cannot be flat.", m.path)
	}
	if !stat.IsDir() {
		return fmt.Errorf("%s is not a directory, metadata cannot be flat.", m.path)
	}
	stat, err = os.Stat(filepath.Join(m.path, ".git"))
	if err == nil {
		return fmt.Errorf("%s has a .git directory, metadata cannot be flat.", m.path)
	}
	for _, g := range [...]string{"*/", "feature/*/"} {
		glob := filepath.Join(m.path, g)
		releases, err := filepath.Glob(glob)
		dieIfError(err)
		for _, rel := range releases {
			rel = strings.Trim(strings.TrimPrefix(rel, m.path), "/")
			m.releases[rel] = m.populateRelease(rel)
		}
	}
	return nil
}

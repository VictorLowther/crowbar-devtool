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

// Base type for representing flat metadata.
type FlatMetadata struct {
	path     string
	releases map[string]*FlatRelease
	crowbar  *Crowbar
}

// How we represent a release in the flat metadata.
type FlatRelease struct {
	name, parent string
	meta         *FlatMetadata
	builds       map[string]*FlatBuild
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
func (r *FlatRelease) Builds() (res BuildMap) {
	res = make(BuildMap)
	if r.builds == nil {
		log.Panicf("Release %s has no builds.", r.name)
	}
	for name,build := range r.builds {
		res[name]=build
	}
	return res
}

func (r *FlatRelease) lookupParent() (res *FlatRelease) {
	if r.parent == "" {
		return nil
	}
	if res := r.meta.releases[r.parent]; res != nil {
		return res
	}
	log.Panicf("Parent release %s of %s does not exist!", r.parent, r.name)
	return nil
}

// Find the parent release of this release.
// If there isn't one, return nil.
func (r *FlatRelease) Parent() Release {
	return Release(r.lookupParent())
}

// Sets target to be the new parent of r.
func (r *FlatRelease) SetParent(target *FlatRelease) error {
	buf := bytes.NewBufferString(target.name)
	if err := ioutil.WriteFile(filepath.Join(r.path(),"parent"),
		buf.Bytes(),
		os.FileMode(0644)); err != nil {
		return err
	}
	relpath := r.meta.crowbar.RelPath(r.path())
	cmd,_,_ := r.meta.crowbar.Repo.Git("add",relpath)
	if err := cmd.Run(); err != nil {
		return err
	}
	commitmsg := fmt.Sprint("Set parent of %s to %s",r.name,target.name)
	cmd,_,_ = r.meta.crowbar.Repo.Git("commit","-m",commitmsg)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}


// Zap a release.  It will reparent any child releases.
func (r *FlatRelease) Zap() error {
	for _, release := range r.meta.releases {
		if release.parent == r.name {
			// Reparent any child releases.
			release.SetParent(r.lookupParent())
		}
	}
	relpath := r.meta.crowbar.RelPath(r.path())
	cmd, _, _ := r.meta.crowbar.Repo.Git("rm", "-rf", relpath)
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd, _, _ = r.meta.crowbar.Repo.Git("commit", "-m", "Removed release "+r.Name())
	if err := cmd.Run(); err != nil {
		return err
	}
	delete(r.meta.releases,r.name)
	r = nil
	return nil
}

// How we represent a build in the flat metadata.
type FlatBuild struct {
	name, parent string
	release      *FlatRelease
	barclamps    BarclampMap
}

// Return the absolute path to the location that the build metadata is at.
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
	return Release(b.release)
}

// The parent build of this one.
func (b *FlatBuild) Parent() Build {
	if b.parent == "" {
		return nil
	}
	if res := b.release.builds[b.parent]; res != nil {
		return Build(res)
	}
	log.Panicf("Release %s: cannot find parent build %s of build %s!",
		b.release.name, b.parent, b.name)
	return nil
}

// The barclamps that are a part of this build.
func (b *FlatBuild) Barclamps() BarclampMap {
	return b.barclamps
}

// Perform switch finalization for FlatMetadata.
// Currently, this involves recreating the extras and change-image symlinks.
func (b *FlatBuild) FinalizeSwitch() {
	pwd, err := os.Getwd()
	if err != nil {
		log.Panic(err)
	}
	defer os.Chdir(pwd)
	os.Chdir(b.release.meta.crowbar.Repo.WorkDir)
	for _, link := range []string{"change-image", "extra"} {
		os.Remove(link)
		os.Symlink(filepath.Join(b.path(), link), link)
	}
}

// Zap a build.  This erases the build metadata from the disk.
func (b *FlatBuild) Zap() error {
	for _, build := range b.release.builds {
		if build.parent == b.name {
			return fmt.Errorf("Cannot delete build with active children!")
		}
	}
	cb_path := filepath.Clean(b.release.meta.crowbar.Repo.WorkDir) + "/"
	relpath := strings.TrimPrefix(b.path(), cb_path)
	cmd, _, _ := b.release.meta.crowbar.Repo.Git("rm", "-rf", relpath)
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd, _, _ = b.release.meta.crowbar.Repo.Git("commit", "-m", "Removed build "+b.FullName())
	if err := cmd.Run(); err != nil {
		return err
	}
	delete(b.release.builds,b.name)
	b = nil
	return nil
}

// Get a list of releases that this metadata knows about
func (m *FlatMetadata) Releases() ReleaseMap {
	res := make(ReleaseMap)
	if m.releases == nil {
		log.Panicf("No releases available")
	}
	for name,rel := range m.releases {
		res[name]=rel
	}
	return res
}

// Get a list of all the builds this metadata knows about.
// The returned BuildMap will have build.FullName() keys.
func (m *FlatMetadata) AllBuilds() map[string]*FlatBuild {
	res := make(map[string]*FlatBuild)
	for _, rel := range m.releases {
		for _, bld := range rel.builds {
			res[bld.FullName()] = bld
		}
	}
	return res
}

func (m *FlatMetadata) populateBuild(release *FlatRelease, name string) *FlatBuild {
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
		builds: make(map[string]*FlatBuild),
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
	m.releases = make(map[string]*FlatRelease)
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

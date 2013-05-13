package devtool

import (
	"os"
	"path/filepath"
	"strings"
	"io/ioutil"
	"fmt"
	"bytes"
)

// Holds important information for flat metdata
type FlatMetadata struct {
	path string
	releases ReleaseMap
	crowbar *Crowbar
}

type FlatRelease struct {
	name,parent string
	meta *FlatMetadata
	builds BuildMap
}

func (r *FlatRelease) path() (string) {
	return filepath.Join(r.meta.path,r.name)
}

func (r *FlatRelease) Name() (string) {
	return r.name
}

func (r *FlatRelease) Builds() (BuildMap,error) {
	if r.builds == nil {
		return nil,fmt.Errorf("Release %s has no builds.",r.name)
	}
	return r.builds,nil
}

func (r *FlatRelease) Parent() (Release,error) {
	if r.parent == "" {
		return nil,nil
	}
	if r.meta.releases[r.parent] == nil {
		return nil,fmt.Errorf("Parent release %s of %s does not exist!",r.parent,r.name)
	}
	return r.meta.releases[r.parent],nil
}

type FlatBuild struct {
	name,parent string
	release *FlatRelease
	barclamps BarclampMap
}

func (b *FlatBuild) path() (string) {
	return filepath.Join(b.release.path(),b.name)
}

func (b *FlatBuild) Name() (string) {
	return b.name
}

func (b *FlatBuild) Release() (Release,error) {
	if b.release == nil {
		return nil,fmt.Errorf("Build %s is not a member of any release!",b.name)
	}
	return b.release,nil
}

func (b *FlatBuild) Parent() (Build,error) {
	if b.parent == "" {
		return nil, nil
	}
	if b.release.builds[b.parent] == nil {
		return nil,fmt.Errorf("Release %s: cannot find parent build %s of build %s!",
			b.release.name,b.parent,b.name)
	}
	return b.release.builds[b.parent],nil
}

func (b *FlatBuild) Barclamps() (BarclampMap,error) {
	return b.barclamps,nil
}


// Get a list of releases in Crowbar
func (m *FlatMetadata) Releases() (ReleaseMap, error) {
	if m.releases == nil {
		return nil,fmt.Errorf("No releases available")
	}
	return m.releases,nil
}

func (m *FlatMetadata) populateBuild(release *FlatRelease, name string) (Build, error) {
	build := &FlatBuild{
		name: name,
		release: release,
		barclamps: make(BarclampMap),
	}
	bld := build.path()
	glob := filepath.Join(bld,"barclamp-*")
	barclamps,err := filepath.Glob(glob)
	c := release.meta.crowbar
	if err != nil {
		return nil,err
	}
	for _,bc := range barclamps {
		barclamp := &Barclamp{}
		barclamp.Name = strings.TrimPrefix(bc,filepath.Join(bld,"barclamp-"))
		if c.Barclamps[barclamp.Name] == nil {
			return nil, fmt.Errorf("Build %s/%s wants %s, which is not in %s\n",
				release.name,
				build.name,
				barclamp.Name,
				filepath.Join(c.Repo.Path(),
				"barclamps",
				barclamp.Name))
		}
		barclamp.Repo = c.Barclamps[barclamp.Name]
		branch,err := ioutil.ReadFile(bc)
		if err != nil {
			continue
		}
		buf := bytes.NewBuffer(branch)
		barclamp.Branch = strings.TrimSpace(buf.String())
		build.barclamps[barclamp.Name]=barclamp
	}
	return build,nil
}

func (m *FlatMetadata) populateRelease(rel string) (*FlatRelease, error) {
	release := &FlatRelease{
		meta: m,
		name: rel,
		builds: make(BuildMap),
	}
	prefix := release.path()
	glob := filepath.Join(prefix,"*/")
	builds,err := filepath.Glob(glob)
	if err != nil {
		return nil,err
	}
	for _,bld := range builds {
		bld = strings.Trim(strings.TrimPrefix(bld,prefix),"/")
		build, err := m.populateBuild(release,bld)
		if err != nil {
			return nil,err
		}
		release.builds[bld]=build
	}
	return release,nil
}

// Populate the Releases field of a Crowbar struct
func (m *FlatMetadata) Probe(c *Crowbar) (err error) {
	m.path = filepath.Join(m.crowbar.Repo.Path(), "releases")
	m.crowbar = c
	stat,err := os.Lstat(m.path)
	if err != nil {
		return fmt.Errorf("Cannot find %s, metadata cannot be flat.",m.path)
	}
	if !stat.IsDir() {
		return fmt.Errorf("%s is not a directory, metadata cannot be flat.",m.path)
	}
	stat,err = os.Stat(filepath.Join(m.path,".git"))
	if err == nil {
		return fmt.Errorf("%s has a .git directory, metadata cannot be flat.",m.path)
	}
	for _, g := range [...]string{"*/", "feature/*/"} {
		glob := filepath.Join(m.path,g)
		releases,err := filepath.Glob(glob)
		if err != nil {
			return err
		}
		for _,rel := range releases {
			rel = strings.Trim(strings.TrimPrefix(rel,m.path),"/")
			release,err := m.populateRelease(rel)
			if err != nil {
				return err
			}
			m.releases[rel]=release
		}
	}
	return nil
}

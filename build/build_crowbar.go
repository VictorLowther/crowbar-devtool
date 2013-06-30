// build implements various build-related sanity tests.
package build

import (
	"fmt"
	"io/ioutil"
	"launchpad.net/goyaml"
	"log"
	"path/filepath"
	"sort"
	"strings"
)

var NotABarclamp error = fmt.Errorf("Not a barclamp")

// Barclamp tracks the barclamp section of the crowbar.yml file.
type BarclampSection struct {
	Name       string
	Display    string
	OnlineHelp string `yaml:"online_help"`
	Version    int
	Member     []string
	Supercedes []string
	Requires   []string
	OsSupport  []string `yaml:"os_support"`
}

// Packages holds OS intepedent parts of the packaging
// dependencies for a barclamp
type PackagesSection struct {
	Repos         []string
	Packages      []string `yaml:"pkgs"`
	BuildPackages []string `yaml:"build_pkgs"`
}

// Debs tracks Debian package requirements.
type DebsSection struct {
	PackagesSection
	Ubuntu1204 *PackagesSection `yaml:"ubuntu-12.04"`
}

// Rpms tracks RPM specific package requirements.
type RpmsSection struct {
	PackagesSection
	Redhat64 *PackagesSection `yaml:"redhat-6.4"`
	Centos64 *PackagesSection `yaml:"centos-6.4"`
}

// BarclampMeta holds all of the metadata from crowbar.yml we care about.
type CrowbarYML struct {
	Barclamp   *BarclampSection
	Debs       *DebsSection
	Rpms       *RpmsSection
	Gems       *PackagesSection
	sortedDeps []string
	ExtraFiles []string `yaml:"extra_files"`
	GitRepos   []string `yaml:"git_repo"`
}

// For dependency sorting barclamps.
type CrowbarYMLs []*CrowbarYML

func (s CrowbarYMLs) Len() int {
	return len(s)
}

func (s CrowbarYMLs) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// s[i] is less than s[j] iff s[j] depends on s[i]
func (s CrowbarYMLs) Less(i, j int) bool {
	return len(s[i].deps()) < len(s[j].deps())
}

func (c *CrowbarYML) String() string {
	return fmt.Sprintf("%s: %v",c.Barclamp.Name,c.deps())
}

// Get the ordered list of dependencies for this barclamp metadata.
// All barclamps except for the crowbar barclamp get a free dependency on the
// crowbar barclamp.
// This should not be called until after we have established the overall global
// barclamp order, which will catch missing barclamps and circular dependencies.
func (b *CrowbarYML) deps() []string {
	if b.Barclamp.Name == "crowbar" {
		// special case: crowbar barclamp has no dependencies.
		return make([]string,0,0)
	}
	if len(b.sortedDeps) > 0 {
		return b.sortedDeps
	}
	// Everyone who is not Crowbar depends on Crowbar
	found_deps := make(map[int][]string)
	found_deps[1]=[]string{"crowbar"}
	// Find all our dependencies, sorted by how many dependencies they have.
	for _, n := range b.Barclamp.Requires {
		bc,ok := allMetadata[n]
		// If we cannot find metadata for n, we are doomed.
		// Doomed.
		if !ok {
			log.Fatalf("%s has %s in its dependency chain, but %s does not exist!", b.Barclamp.Name, n, n)
		}
		t := append(bc.deps(),n)
		rank := len(t)
		_, ok = found_deps[rank]
		if !ok {
			found_deps[rank] = t
		} else {
			found_deps[rank] = append(found_deps[rank], t...)
		}
		// If our name is in our list of dependencies, then there is a
		// curcular dependency.  Handle it by dying horribly.
		for _,k := range t {
			if k == b.Barclamp.Name {
				log.Fatalf("%s cannot depend on itself!",k)
			}
		}
	}
	// Figure out what ranks we have, then sort them.
	ranks := make([]int, 0, len(found_deps))
	for i := range found_deps {
		ranks = append(ranks, i)
		// Sorting here ensures that our overall global order will
		// remain consistent.
		sort.Strings(found_deps[i])
	}
	sort.Ints(ranks)
	// Figure out the overall global ordering and remember it.
	final_deps := make(map[string]int)
	final_rank := int(0)
	for _, rank := range ranks {
		for _, dep := range found_deps[rank] {
			if _, ok := final_deps[dep]; !ok {
				final_deps[dep] = final_rank
				final_rank += 1
			}
		}
	}
	// Invert final rank directly into res
	res := make([]string, final_rank, final_rank)
	for dep,i := range final_deps {
		res[i] = dep
	}
	b.sortedDeps = res
	return res
}

type group struct {
	name    string
	members []string
}

var allMetadata map[string]*CrowbarYML = make(map[string]*CrowbarYML)
var sortedMetadata CrowbarYMLs = make(CrowbarYMLs, 0, 0)
var groups map[string]*group = make(map[string]*group)

// Loads (but does not process) metadata for a single barclamp.
func loadOneMeta(path string) (res *CrowbarYML, err error) {
	if filepath.Base(path) != "crowbar.yml" {
		return nil, NotABarclamp
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = goyaml.Unmarshal(data, &res)
	return res, err
}

func SanityCheckMetadata(paths []string) {
	for _, path := range paths {
		bc, err := loadOneMeta(path)
		if err != nil {
			log.Fatalln(err)
		}
		allMetadata[bc.Barclamp.Name] = bc
	}
	// Process supercedes directives first.
	toRemove := make(map[string]bool)
	for name, bc := range allMetadata {
		for _, victim := range bc.Barclamp.Supercedes {
			log.Printf("%s supercedes %s\n", name, victim)
			toRemove[name] = true
		}
	}
	for name := range toRemove {
		delete(allMetadata, name)
	}
	// Once supercedes directives are processed, we can assemble groups
	// from the members sections of the metadata.
	for name, bc := range allMetadata {
		for _, member := range bc.Barclamp.Member {
			grp, ok := groups[member]
			if !ok {
				groups[member] = &group{name: member, members: []string{name}}
			} else {
				grp.members = append(grp.members, name)
			}
		}
	}

	// Once groups are processed, we can expand dependencies.
	for name, bc := range allMetadata {
		newRequires := make([]string, 0, len(bc.Barclamp.Requires))
		for _, requirement := range bc.Barclamp.Requires {
			if strings.HasPrefix(requirement, "@") {
				//This is a group requirement.  Expand it.
				g, ok := groups[strings.TrimPrefix(requirement, "@")]
				if !ok {
					log.Fatalf("%s requires group %s, which does not exist!", name, requirement)
				}
				for _, n := range g.members {
					newRequires = append(newRequires, n)
				}
			} else {
				newRequires = append(newRequires, requirement)
			}
		}
		sort.Strings(newRequires)
		bc.Barclamp.Requires = newRequires
	}
	// Once all dependencies are expanded, we can figure out the global
	// order we will walk over barclamps in.
	for _, bc := range allMetadata {
		sortedMetadata = append(sortedMetadata, bc)
	}
	sort.Sort(sortedMetadata)
}

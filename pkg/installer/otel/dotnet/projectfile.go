package dotnet

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ProjectFile is the statically read content of a project file. MSBuild is not
// evaluated: only unconditional properties of the file itself are read, so
// values inherited from shared build files (Directory.Build.props) are absent.
type ProjectFile struct {
	Path          string
	Name          string   // file name without extension
	AssemblyName  string   // empty when not declared
	Frameworks    []string // empty when not declared (unknown)
	IsService     bool     // executable, web or worker project that is not a test project
	IsTest        bool
	LegacyFormat  bool // non-SDK style project file
	SelfContained bool // self-contained or native-AOT publishing
	// DotNetFramework is true when every target framework is .NET Framework.
	DotNetFramework bool
}

type msbuildProject struct {
	XMLName xml.Name
	Sdk     string `xml:"Sdk,attr"`
	Sdks    []struct {
		Name string `xml:"Name,attr"`
	} `xml:"Sdk"`
	Imports []struct {
		Sdk string `xml:"Sdk,attr"`
	} `xml:"Import"`
	PropertyGroups []struct {
		Condition  string `xml:"Condition,attr"`
		Properties []struct {
			XMLName   xml.Name
			Condition string `xml:"Condition,attr"`
			Value     string `xml:",chardata"`
		} `xml:",any"`
	} `xml:"PropertyGroup"`
	ItemGroups []struct {
		PackageReferences []struct {
			Include string `xml:"Include,attr"`
		} `xml:"PackageReference"`
	} `xml:"ItemGroup"`
}

var netFrameworkMoniker = regexp.MustCompile(`^net\d+$`)

// ParseProjectFile reads a project file and classifies it.
func ParseProjectFile(path string) (ProjectFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ProjectFile{}, fmt.Errorf("reading project file: %w", err)
	}
	var doc msbuildProject
	if err := xml.Unmarshal(data, &doc); err != nil {
		return ProjectFile{}, fmt.Errorf("parsing project file %s: %w", path, err)
	}
	if doc.XMLName.Local != "Project" {
		return ProjectFile{}, fmt.Errorf("parsing project file %s: root element is %q, want Project", path, doc.XMLName.Local)
	}

	props := map[string]string{}
	for _, group := range doc.PropertyGroups {
		if group.Condition != "" {
			continue
		}
		for _, p := range group.Properties {
			if p.Condition != "" {
				continue
			}
			key := strings.ToLower(p.XMLName.Local)
			if _, seen := props[key]; !seen {
				props[key] = strings.TrimSpace(p.Value)
			}
		}
	}

	sdks := []string{doc.Sdk}
	for _, s := range doc.Sdks {
		sdks = append(sdks, s.Name)
	}
	for _, i := range doc.Imports {
		sdks = append(sdks, i.Sdk)
	}
	hasSdk, isWeb, isWorker, isTestSdk := false, false, false, false
	for _, s := range sdks {
		lower := strings.ToLower(s)
		if lower == "" {
			continue
		}
		hasSdk = true
		isWeb = isWeb || strings.Contains(lower, "microsoft.net.sdk.web") && !strings.Contains(lower, "blazorwebassembly")
		isWorker = isWorker || strings.Contains(lower, "microsoft.net.sdk.worker")
		isTestSdk = isTestSdk || strings.HasPrefix(lower, "mstest.sdk")
	}

	pf := ProjectFile{
		Path:         path,
		Name:         strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		AssemblyName: props["assemblyname"],
		LegacyFormat: !hasSdk,
	}

	pf.Frameworks = splitFrameworks(props["targetframeworks"])
	if len(pf.Frameworks) == 0 {
		pf.Frameworks = splitFrameworks(props["targetframework"])
	}
	if len(pf.Frameworks) == 0 && pf.LegacyFormat {
		if v := strings.TrimPrefix(strings.ToLower(props["targetframeworkversion"]), "v"); v != "" {
			pf.Frameworks = []string{"net" + strings.ReplaceAll(v, ".", "")}
		}
	}
	pf.DotNetFramework = pf.LegacyFormat
	if !pf.LegacyFormat && len(pf.Frameworks) > 0 {
		pf.DotNetFramework = true
		for _, f := range pf.Frameworks {
			if !netFrameworkMoniker.MatchString(strings.ToLower(f)) {
				pf.DotNetFramework = false
			}
		}
	}

	pf.SelfContained = isTrue(props["selfcontained"]) || isTrue(props["publishaot"])

	pf.IsTest = isTestSdk || isTrue(props["istestproject"])
	for _, ig := range doc.ItemGroups {
		for _, ref := range ig.PackageReferences {
			if strings.EqualFold(ref.Include, "Microsoft.NET.Test.Sdk") {
				pf.IsTest = true
			}
		}
	}

	outputType := strings.ToLower(props["outputtype"])
	switch outputType {
	case "exe", "winexe":
		pf.IsService = true
	case "library":
		pf.IsService = false
	default:
		pf.IsService = isWeb || isWorker
	}
	if pf.IsTest {
		pf.IsService = false
	}
	return pf, nil
}

func splitFrameworks(value string) []string {
	var out []string
	for _, f := range strings.Split(value, ";") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func isTrue(v string) bool { return strings.EqualFold(strings.TrimSpace(v), "true") }

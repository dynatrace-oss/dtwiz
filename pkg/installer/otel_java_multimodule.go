package installer

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

type SubModule struct {
	Name string
	Path string
}

type MultiModuleProject struct {
	BuildTool    string
	Modules      []SubModule
	BuildCommand string
}

type mavenPOM struct {
	Modules []string `xml:"modules>module"`
}

var gradleIncludeRe = regexp.MustCompile(`(?m)include\s*\(?\s*['"]([^'"]+)['"]`)

func parseMavenModules(projectPath string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(projectPath, "pom.xml"))
	if err != nil {
		return nil, err
	}
	var pom mavenPOM
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil, fmt.Errorf("parsing pom.xml: %w", err)
	}
	return pom.Modules, nil
}

func isMavenMultiModule(projectPath string) bool {
	modules, err := parseMavenModules(projectPath)
	return err == nil && len(modules) > 0
}

// converts Gradle colon notation to path separators (e.g. ":ui:web" → "ui/web").
func parseGradleSubprojects(projectPath string) ([]string, error) {
	var data []byte
	var readErr error
	for _, name := range []string{"settings.gradle", "settings.gradle.kts"} {
		data, readErr = os.ReadFile(filepath.Join(projectPath, name))
		if readErr == nil {
			break
		}
	}
	if readErr != nil {
		return nil, readErr
	}
	matches := gradleIncludeRe.FindAllSubmatch(data, -1)
	var result []string
	for _, m := range matches {
		path := strings.TrimPrefix(string(m[1]), ":")
		path = strings.ReplaceAll(path, ":", "/")
		result = append(result, path)
	}
	return result, nil
}

func isGradleMultiProject(projectPath string) bool {
	subs, err := parseGradleSubprojects(projectPath)
	return err == nil && len(subs) > 0
}

func mavenBuildCommand(projectPath string) string {
	mvnCmd, _ := resolveMavenCmd(projectPath)
	if mvnCmd == "" {
		return ""
	}
	return mvnCmd + " clean package -DskipTests"
}

func gradleBuildCommand(projectPath string) string {
	gradleCmd, _ := resolveGradleCmd(projectPath)
	if gradleCmd == "" {
		return ""
	}
	return gradleCmd + " build -x test"
}

func hasExecutableJar(projectPath string) bool {
	for _, dir := range []string{
		filepath.Join(projectPath, "target"),
		filepath.Join(projectPath, "build", "libs"),
	} {
		if !fileExists(dir) {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".jar") {
				if isExecutableJar(filepath.Join(dir, e.Name())) {
					return true
				}
			}
		}
	}
	return false
}

func needsBuild(subs []SubModule) bool {
	for _, sub := range subs {
		if !hasExecutableJar(sub.Path) {
			return true
		}
	}
	return false
}

// Maven is checked before Gradle — pom.xml takes precedence when both exist.
func detectMultiModule(projectPath string) *MultiModuleProject {
	modules, err := parseMavenModules(projectPath)
	if err == nil && len(modules) > 0 {
		subs := make([]SubModule, len(modules))
		for i, mod := range modules {
			subs[i] = SubModule{
				Name: mod,
				Path: filepath.Join(projectPath, mod),
			}
		}
		logger.Debug("detected maven multi-module project", "modules", len(subs))
		return &MultiModuleProject{
			BuildTool:    "maven",
			Modules:      subs,
			BuildCommand: mavenBuildCommand(projectPath),
		}
	}

	if !isGradleMultiProject(projectPath) {
		return nil
	}
	subprojects, err := parseGradleSubprojects(projectPath)
	if err == nil && len(subprojects) > 0 {
		subs := make([]SubModule, len(subprojects))
		for i, sub := range subprojects {
			subs[i] = SubModule{
				Name: filepath.Base(sub),
				Path: filepath.Join(projectPath, filepath.FromSlash(sub)),
			}
		}
		logger.Debug("detected gradle multi-module project", "modules", len(subs))
		return &MultiModuleProject{
			BuildTool:    "gradle",
			Modules:      subs,
			BuildCommand: gradleBuildCommand(projectPath),
		}
	}

	return nil
}

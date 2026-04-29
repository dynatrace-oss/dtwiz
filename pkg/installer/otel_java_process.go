package installer

import (
	"archive/zip"
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

type JavaEntrypoint struct {
	Command     string
	Description string
}

var javaVersionRegexp = regexp.MustCompile(`version "([^"]+)"`)

// parseJavaVersion extracts the major version integer from `java -version` output.
// It handles legacy "1.8.0_382" format (returns 8) and modern "17.0.1" / "21" formats.
func parseJavaVersion(output string) (int, error) {
	m := javaVersionRegexp.FindStringSubmatch(output)
	if m == nil {
		return 0, fmt.Errorf("could not parse Java version from output: %q", output)
	}
	versionStr := m[1]
	parts := strings.Split(versionStr, ".")
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid Java version component %q: %w", parts[0], err)
	}

	// Legacy format: "1.8.0_382" — first component is "1", use the second.
	if major == 1 && len(parts) >= 2 {
		minor, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid Java version minor component %q: %w", parts[1], err)
		}
		logger.Debug("java version parsed", "raw", versionStr, "major", minor)
		return minor, nil
	}

	logger.Debug("java version parsed", "raw", versionStr, "major", major)
	return major, nil
}

// validateJavaPrerequisites checks that java is available and meets the minimum
// version requirement (Java 8+). Returns the path to the java binary on success.
func validateJavaPrerequisites() (string, error) {
	javaPath, err := exec.LookPath("java")
	if err != nil {
		display.PrintStatusLine("error", "Java not found — install a JDK/JRE and ensure it is in PATH", display.ColorError)
		return "", fmt.Errorf("Java not found on PATH") //nolint:staticcheck // ST1005: keep brand capitalization
	}
	logger.Debug("java binary found", "path", javaPath)

	out, err := exec.Command(javaPath, "-version").CombinedOutput()
	if err != nil {
		display.PrintStatusLine("error", fmt.Sprintf("unable to determine Java version: %v", err), display.ColorError)
		return "", fmt.Errorf("unable to determine Java version: %w", err)
	}

	major, err := parseJavaVersion(string(out))
	if err != nil {
		display.PrintStatusLine("error", fmt.Sprintf("could not parse Java version: %v", err), display.ColorError)
		return "", err
	}

	if major < 8 {
		logger.Debug("java version too old", "major", major, "minimum", 8)
		err := fmt.Errorf("Java %d is too old — Java 8 or newer is required", major) //nolint:staticcheck
		display.PrintStatusLine("error", err.Error(), display.ColorError)
		return "", err
	}

	logger.Debug("java version OK", "major", major)
	return javaPath, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// findWrapper returns the wrapper filename (not full path) for the current platform.
// On Windows it checks windowsName; on all other platforms it checks unixName.
// Returns "" if the file does not exist.
func findWrapper(projectPath, unixName, windowsName string) string {
	if runtime.GOOS == "windows" {
		if fileExists(filepath.Join(projectPath, windowsName)) {
			return windowsName
		}
		return ""
	}
	if fileExists(filepath.Join(projectPath, unixName)) {
		return unixName
	}
	return ""
}

// isExecutableJar returns true if the JAR at jarPath has a Main-Class entry in
// META-INF/MANIFEST.MF.
func isExecutableJar(jarPath string) bool {
	r, err := zip.OpenReader(jarPath)
	if err != nil {
		return false
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name != "META-INF/MANIFEST.MF" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return false
		}
		defer rc.Close()
		scanner := bufio.NewScanner(rc)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "Main-Class:") {
				return true
			}
		}
		return false
	}
	return false
}

func isSpringBootMaven(projectPath string) bool {
	filePath := filepath.Join(projectPath, "pom.xml")
	data, err := os.ReadFile(filePath)
	if err != nil {
		logger.Debug("Spring Boot detection", "file", filePath, "result", false)
		return false
	}
	result := strings.Contains(string(data), "spring-boot")
	logger.Debug("Spring Boot detection", "file", filePath, "result", result)
	return result
}

func isSpringBootGradle(projectPath string) bool {
	for _, name := range []string{"build.gradle", "build.gradle.kts"} {
		filePath := filepath.Join(projectPath, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		content := string(data)
		result := strings.Contains(content, "spring-boot") || strings.Contains(content, "springframework.boot")
		logger.Debug("Spring Boot detection", "file", filePath, "result", result)
		if result {
			return true
		}
	}
	return false
}

func resolveMavenCmd(projectPath string) (cmd, desc string) {
	if !fileExists(filepath.Join(projectPath, "pom.xml")) {
		return "", ""
	}
	wrapperName := findWrapper(projectPath, "mvnw", "mvnw.cmd")
	if wrapperName != "" && fileExists(filepath.Join(projectPath, ".mvn", "wrapper", "maven-wrapper.jar")) {
		if runtime.GOOS == "windows" {
			return wrapperName, "Maven"
		}
		return "./" + wrapperName, "Maven"
	}
	if _, err := exec.LookPath("mvn"); err == nil {
		return "mvn", "Maven"
	}
	if wrapperName != "" {
		display.PrintStatusLine("error", "maven-wrapper.jar not found and 'mvn' is not in PATH — install Maven or run: mvn wrapper:wrapper", display.ColorError)
	}
	return "", ""
}

func resolveGradleCmd(projectPath string) (cmd, desc string) {
	hasBuildFile := fileExists(filepath.Join(projectPath, "build.gradle")) ||
		fileExists(filepath.Join(projectPath, "build.gradle.kts"))
	if !hasBuildFile {
		return "", ""
	}
	wrapperName := findWrapper(projectPath, "gradlew", "gradlew.bat")
	if wrapperName != "" && fileExists(filepath.Join(projectPath, "gradle", "wrapper", "gradle-wrapper.jar")) {
		if runtime.GOOS == "windows" {
			return wrapperName, "Gradle"
		}
		return "./" + wrapperName, "Gradle"
	}
	if _, err := exec.LookPath("gradle"); err == nil {
		return "gradle", "Gradle"
	}
	if wrapperName != "" {
		display.PrintStatusLine("error", "gradle-wrapper.jar not found and 'gradle' is not in PATH — install Gradle or run: gradle wrapper --gradle-version 8.7", display.ColorError)
	}
	return "", ""
}

func detectJavaEntrypoints(projectPath string) []JavaEntrypoint {
	var entrypoints []JavaEntrypoint
	var scanned []string

	jarDirs := []string{
		filepath.Join(projectPath, "target"),
		filepath.Join(projectPath, "build", "libs"),
	}

	foundJar := false
	for _, dir := range jarDirs {
		scanned = append(scanned, dir)
		if !fileExists(dir) {
			logger.Debug("dir not found, skipping JAR scan", "dir", dir)
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jar") {
				continue
			}
			jarPath := filepath.Join(dir, entry.Name())
			if isExecutableJar(jarPath) {
				logger.Debug("executable JAR found", "jar", jarPath)
				entrypoints = append(entrypoints, JavaEntrypoint{
					Command:     "java -jar " + jarPath,
					Description: entry.Name(),
				})
				foundJar = true
			} else {
				logger.Debug("skipping JAR — no Main-Class in MANIFEST.MF", "jar", jarPath)
			}
		}
	}

	if foundJar {
		if len(entrypoints) == 1 {
			logger.Debug("auto-selected single entrypoint", "command", entrypoints[0].Command)
		}
		return entrypoints
	}

	mvnCmd, mvnDesc := resolveMavenCmd(projectPath)
	gradleCmd, gradleDesc := resolveGradleCmd(projectPath)

	if mvnCmd != "" {
		if isSpringBootMaven(projectPath) {
			cmd := mvnCmd + " spring-boot:run"
			logger.Debug("no fat JAR found, using maven fallback", "command", cmd)
			entrypoints = append(entrypoints, JavaEntrypoint{Command: cmd, Description: mvnDesc + " Spring Boot"})
		} else {
			cmd := mvnCmd + " exec:java"
			logger.Debug("no fat JAR found, using maven fallback", "command", cmd)
			entrypoints = append(entrypoints, JavaEntrypoint{Command: cmd, Description: mvnDesc + " run"})
		}
	} else if gradleCmd != "" {
		if isSpringBootGradle(projectPath) {
			cmd := gradleCmd + " bootRun"
			logger.Debug("no fat JAR found, using gradle fallback", "command", cmd)
			entrypoints = append(entrypoints, JavaEntrypoint{Command: cmd, Description: gradleDesc + " Spring Boot"})
		} else {
			cmd := gradleCmd + " run"
			logger.Debug("no fat JAR found, using gradle fallback", "command", cmd)
			entrypoints = append(entrypoints, JavaEntrypoint{Command: cmd, Description: gradleDesc + " run"})
		}
	}

	if len(entrypoints) == 0 {
		logger.Debug("no entrypoint found", "project", projectPath, "scanned", strings.Join(scanned, ", "))
	} else if len(entrypoints) == 1 {
		logger.Debug("auto-selected single entrypoint", "command", entrypoints[0].Command)
	}

	return entrypoints
}

func attemptSingleModuleBuild(projectPath string) error {
	mvnCmd, _ := resolveMavenCmd(projectPath)
	gradleCmd, _ := resolveGradleCmd(projectPath)

	var cmd *exec.Cmd
	var displayCmd string

	switch {
	case mvnCmd != "":
		displayCmd = mvnCmd + " clean package -DskipTests"
		if strings.HasSuffix(mvnCmd, ".cmd") {
			cmd = exec.Command("cmd", "/c", mvnCmd, "clean", "package", "-DskipTests")
		} else {
			cmd = exec.Command(mvnCmd, "clean", "package", "-DskipTests")
		}
	case gradleCmd != "":
		displayCmd = gradleCmd + " build -x test"
		if strings.HasSuffix(gradleCmd, ".bat") {
			cmd = exec.Command("cmd", "/c", gradleCmd, "build", "-x", "test")
		} else {
			cmd = exec.Command(gradleCmd, "build", "-x", "test")
		}
	default:
		display.PrintStatusLine("error", "no build tool detected — build the project manually and re-run", display.ColorError)
		return fmt.Errorf("no build tool detected")
	}

	logger.Debug("attempting auto-build", "command", displayCmd, "project", projectPath)
	cmd.Dir = projectPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Debug("auto-build failed", "project", projectPath, "error", err)
		return fmt.Errorf("auto-build failed: %w", err)
	}

	logger.Debug("auto-build succeeded", "project", projectPath)
	return nil
}

// promptEntrypointSelection presents a selection menu for available entrypoints.
// Returns nil if the user skips.
func promptEntrypointSelection(entrypoints []JavaEntrypoint) *JavaEntrypoint {
	if len(entrypoints) == 1 {
		display.PrintStatusLine("entrypoint", entrypoints[0].Command, display.ColorOK)
		return &entrypoints[0]
	}

	fmt.Println()
	display.ColorHeader.Println("  Available entrypoints:")
	display.PrintSectionDivider()
	for i, ep := range entrypoints {
		if ep.Description != "" {
			fmt.Printf("  [%d]  %s  (%s)\n", i+1, ep.Command, ep.Description)
		} else {
			fmt.Printf("  [%d]  %s\n", i+1, ep.Command)
		}
	}
	fmt.Println()
	fmt.Printf("  Select an entrypoint [1-%d] or press Enter to skip: ", len(entrypoints))

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if answer == "" {
		logger.Debug("user skipped entrypoint selection")
		return nil
	}

	selection, err := strconv.Atoi(answer)
	if err != nil || selection < 1 || selection > len(entrypoints) {
		logger.Debug("invalid entrypoint selection, skipping", "input", answer)
		fmt.Println("  Invalid selection, skipping.")
		return nil
	}
	logger.Debug("user selected entrypoint", "command", entrypoints[selection-1].Command)
	return &entrypoints[selection-1]
}

// detectJavaListeningPort extends detectProcessListeningPort with a jps -l
// fallback that skips build-tool JVMs. Needed for Gradle, where the app runs
// inside the daemon rather than the tracked wrapper process.
func detectJavaListeningPort(pid int) string {
	if port := detectProcessListeningPort(pid); port != "" {
		return port
	}
	out, err := exec.Command("jps", "-l").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 2 {
			continue
		}
		jvmPID, err := strconv.Atoi(fields[0])
		if err != nil || jvmPID == pid {
			continue
		}
		if isBuildToolJVM(fields[1]) {
			logger.Debug("skipping build-tool JVM", "pid", jvmPID, "class", fields[1])
			continue
		}
		if port := detectProcessListeningPort(jvmPID); port != "" {
			logger.Debug("port found on app JVM", "root_pid", pid, "jvm_pid", jvmPID, "class", fields[1], "port", port)
			return port
		}
	}
	return ""
}

// isBuildToolJVM reports whether a JVM main class belongs to a build-tool process.
func isBuildToolJVM(mainClass string) bool {
	for _, prefix := range []string{
		"org.gradle.",
		"org.apache.maven.",
		"sun.tools.jps.",
		"com.sun.tools.jps.",
	} {
		if strings.HasPrefix(mainClass, prefix) {
			return true
		}
	}
	return false
}

// enrichProcessesWithJPS uses jps -l to populate Description fields on matching processes.
func enrichProcessesWithJPS(processes []DetectedProcess) []DetectedProcess {
	_, err := exec.LookPath("jps")
	if err != nil {
		logger.Debug("jps not found, skipping enrichment")
		return processes
	}

	out, err := exec.Command("jps", "-l").Output()
	if err != nil {
		logger.Debug("jps -l failed", "error", err)
		return processes
	}

	jpsMap := make(map[int]string)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		jpsMap[pid] = fields[1]
	}

	result := make([]DetectedProcess, len(processes))
	copy(result, processes)
	for i := range result {
		if desc, ok := jpsMap[result[i].PID]; ok {
			result[i].Description = desc
			logger.Debug("jps enrichment", "pid", result[i].PID, "description", desc)
		}
	}
	return result
}

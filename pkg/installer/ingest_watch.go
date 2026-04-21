package installer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
	"github.com/fatih/color"
	"golang.org/x/term"
)

const watchPollInterval = 5 * time.Second

// watchSection holds the display data for one section of the watch output.
type watchSection struct {
	Name    string
	Count   int
	Details string // formatted detail line (e.g. service names, type breakdown)
	Link    string // deep link path appended to AppsURL
}

type typeCount struct {
	typeName string
	count    int
}

// watchState holds the aggregated state across all poll cycles.
type watchState struct {
	Services      watchSection
	Cloud         watchSection
	Kubernetes    watchSection
	Relationships watchSection
	Logs          watchSection
	Requests      watchSection
	Exceptions    watchSection
}

// WatchIngest polls Dynatrace for newly ingested data and renders a live
// terminal summary. It blocks until the user presses Ctrl+C.
// fromClause is injected directly into DQL queries — accepts RFC3339 timestamps
// or DQL relative expressions (e.g. "now()-1h").
func WatchIngest(envURL, pToken, fromClause string) {
	if pToken == "" {
		fmt.Println("  Platform token required for watch. Set --platform-token or DT_PLATFORM_TOKEN.")
		return
	}

	appsURL := AppsURL(envURL)
	queryURL := appsURL + "/platform/storage/query/v1/query:execute"
	watchStart := time.Now()
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	// Colors
	highlight := color.New(color.FgMagenta, color.Bold)
	dim := color.New(color.Faint)
	green := color.New(color.FgGreen, color.Bold)
	bold := color.New(color.Bold)
	_ = green

	linkFn := func(url, label string) string {
		return termHyperlink(url, label, isTTY)
	}

	var prevLines int

	// Listen for Enter key in a goroutine to let the user stop watching.
	stopCh := make(chan struct{}, 1)
	if isTTY {
		go func() {
			buf := make([]byte, 1)
			for {
				_, err := os.Stdin.Read(buf)
				if err != nil || buf[0] == '\n' || buf[0] == '\r' {
					stopCh <- struct{}{}
					return
				}
			}
		}()
	}

	ticker := time.NewTicker(watchPollInterval)
	defer ticker.Stop()

	// Run first poll immediately, then on ticker.
	for first := true; ; first = false {
		if !first {
			select {
			case <-ticker.C:
			case <-stopCh:
				return
			}
		}

		elapsed := time.Since(watchStart).Truncate(time.Second)
		state := pollAll(queryURL, pToken, fromClause)

		var buf strings.Builder

		// Header
		highlight.Fprintf(&buf, " Watching for new data in Dynatrace... (elapsed: %s)\n", formatElapsed(elapsed))
		dim.Fprintf(&buf, " Generate some load on your system to see data appear.\n")
		buf.WriteString("\n")

		// Sections
		renderSection(&buf, "Services", state.Services, appsURL, highlight, dim, bold, linkFn)
		renderSection(&buf, "Cloud", state.Cloud, appsURL, highlight, dim, bold, linkFn)
		renderSection(&buf, "Kubernetes", state.Kubernetes, appsURL, highlight, dim, bold, linkFn)
		renderRelationships(&buf, state.Relationships, appsURL, highlight, dim, bold, linkFn)
		renderSection(&buf, "Logs", state.Logs, appsURL, highlight, dim, bold, linkFn)
		renderSection(&buf, "Requests", state.Requests, appsURL, highlight, dim, bold, linkFn)
		renderSection(&buf, "Exceptions", state.Exceptions, appsURL, highlight, dim, bold, linkFn)

		// QuickStart footer
		separator := " ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
		highlight.Fprint(&buf, separator)
		highlight.Fprintf(&buf, " 👉 See all your data and findings in Dynatrace QuickStart\n")
		fmt.Fprintf(&buf, "    %s\n", linkFn(appsURL+"/ui/apps/my.getting.started.dieter/", "→ Open Dynatrace QuickStart"))
		highlight.Fprint(&buf, separator)
		buf.WriteString("\n")
		dim.Fprint(&buf, " Press Enter to continue or keep watching...")
		buf.WriteString("\n")

		output := buf.String()
		lineCount := strings.Count(output, "\n")

		if isTTY && prevLines > 0 {
			// Move cursor up to overwrite previous output
			fmt.Printf("\033[%dA\033[J", prevLines)
		}

		fmt.Print(output)
		prevLines = lineCount

		if !isTTY {
			// Non-TTY: print a separator between updates
			fmt.Println("---")
		}
	}
}

func renderSection(buf *strings.Builder, name string, sec watchSection, appsURL string, highlight, dim, bold *color.Color, linkFn func(string, string) string) {
	if sec.Count > 0 {
		title := name
		if sec.Link != "" {
			title = linkFn(appsURL+sec.Link, name)
		}
		highlight.Fprintf(buf, " %s", title)
		fmt.Fprintf(buf, " (%s)\n", formatCount(sec.Count))
		if sec.Details != "" {
			fmt.Fprintf(buf, "   %s\n", sec.Details)
		}
	} else {
		highlight.Fprintf(buf, " %s\n", name)
		dim.Fprintf(buf, "   waiting...\n")
	}
	buf.WriteString("\n")
}

func renderRelationships(buf *strings.Builder, sec watchSection, appsURL string, highlight, dim, bold *color.Color, linkFn func(string, string) string) {
	if sec.Count > 0 {
		title := "Relationships"
		if sec.Link != "" {
			title = linkFn(appsURL+sec.Link, "Relationships")
		}
		highlight.Fprintf(buf, " %s", title)
		fmt.Fprintf(buf, " (%s)\n", formatCount(sec.Count))
		if sec.Details != "" {
			fmt.Fprintf(buf, "   %s\n", sec.Details)
		}
	} else {
		highlight.Fprintf(buf, " Relationships\n")
		dim.Fprintf(buf, "   waiting...\n")
	}
	buf.WriteString("\n")
}

// pollAll executes all DQL queries in parallel and returns the aggregated state.
// dqlFromLiteral formats a fromClause for use in DQL queries.
// DQL relative expressions (containing parentheses, e.g. "now()-1h") must not
// be quoted; RFC3339 absolute timestamps must be quoted.
func dqlFromLiteral(fromClause string) string {
	for _, ch := range fromClause {
		if ch == '(' || ch == ')' {
			return fromClause
		}
	}
	return `"` + fromClause + `"`
}

func pollAll(queryURL, token string, fromClause string) watchState {
	var state watchState

	type result struct {
		name string
		data *dqlResponse
	}

	from := dqlFromLiteral(fromClause)
	queries := map[string]string{
		"services":      fmt.Sprintf(`smartscapeNodes SERVICE, from:%s | fields name | limit 100`, from),
		"nodes":         fmt.Sprintf(`smartscapeNodes "*", from:%s | summarize count=count(), by:{type} | limit 200`, from),
		"relationships": fmt.Sprintf(`smartscapeEdges "*", from:%s | summarize count=count(), by:{type}`, from),
		"logs":          fmt.Sprintf(`fetch logs, from:%s | summarize count=count(), by:{loglevel}`, from),
		"requests":      fmt.Sprintf(`fetch spans, from:%s | filter request.is_root_span == true | summarize failed=countIf(request.is_failed == true), success=countIf(request.is_failed != true)`, from),
		"exceptions":    fmt.Sprintf(`fetch spans, from:%s | expand events = span.events | filter events[type] == "exception" | summarize count=count()`, from),
	}

	ch := make(chan result, len(queries))
	for name, dql := range queries {
		go func(n, q string) {
			ch <- result{name: n, data: executeDQL(queryURL, token, q)}
		}(name, dql)
	}

	results := make(map[string]*dqlResponse, len(queries))
	for range queries {
		r := <-ch
		results[r.name] = r.data
	}

	// Services
	state.Services = parseServices(results["services"])
	// Cloud + Kubernetes from nodes
	state.Cloud, state.Kubernetes = parseNodes(results["nodes"])
	// Relationships
	state.Relationships = parseRelationships(results["relationships"])
	// Logs
	state.Logs = parseLogs(results["logs"])
	// Requests
	state.Requests = parseRequests(results["requests"])
	// Exceptions
	state.Exceptions = parseExceptions(results["exceptions"])

	return state
}

func executeDQL(queryURL, token, dql string) *dqlResponse {
	payload := map[string]interface{}{
		"query":                      dql,
		"requestTimeoutMilliseconds": 10000,
		"maxResultRecords":           200,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil
	}

	req, err := http.NewRequest(http.MethodPost, queryURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Debug("watch DQL request failed", "err", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Debug("watch DQL non-200", "status", resp.StatusCode, "query", dql)
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	var result dqlResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	return &result
}

func parseServices(resp *dqlResponse) watchSection {
	sec := watchSection{Link: "/ui/apps/dynatrace.services/explorer-new/services-new"}
	if resp == nil || len(resp.Result.Records) == 0 {
		return sec
	}

	var names []string
	for _, rec := range resp.Result.Records {
		if name, ok := rec["name"].(string); ok {
			names = append(names, name)
		}
	}
	sec.Count = len(names)
	if len(names) > 5 {
		sec.Details = strings.Join(names[:5], ", ") + fmt.Sprintf(" +%d more", len(names)-5)
	} else if len(names) > 0 {
		sec.Details = strings.Join(names, ", ")
	}
	return sec
}

func parseNodes(resp *dqlResponse) (cloud, k8s watchSection) {
	cloud = watchSection{Link: "/ui/apps/dynatrace.clouds/smartscape/services"}
	k8s = watchSection{Link: "/ui/apps/dynatrace.kubernetes/smartscape/K8S_CLUSTER"}

	if resp == nil {
		return
	}

	var cloudTypes, k8sTypes []typeCount

	for _, rec := range resp.Result.Records {
		typeName, _ := rec["type"].(string)
		count := toInt(rec["count"])
		if count == 0 {
			continue
		}

		if strings.HasPrefix(typeName, "AWS_") {
			cloudTypes = append(cloudTypes, typeCount{typeName, count})
			cloud.Count += count
		} else if strings.HasPrefix(typeName, "K8S_") || typeName == "CONTAINER" {
			k8sTypes = append(k8sTypes, typeCount{typeName, count})
			k8s.Count += count
		}
	}

	cloud.Details = formatTypeBreakdown(cloudTypes, "AWS_")
	k8s.Details = formatTypeBreakdown(k8sTypes, "K8S_")

	return
}

func parseRelationships(resp *dqlResponse) watchSection {
	sec := watchSection{Link: "/ui/apps/dynatrace.smartscape/view/dynatrace.smartscape.smartscape-on-grail"}
	if resp == nil || len(resp.Result.Records) == 0 {
		return sec
	}

	var types []typeCount
	for _, rec := range resp.Result.Records {
		typeName, _ := rec["type"].(string)
		count := toInt(rec["count"])
		if count == 0 {
			continue
		}
		sec.Count += count
		types = append(types, typeCount{typeName, count})
	}

	if len(types) > 0 {
		sort.Slice(types, func(i, j int) bool { return types[i].count > types[j].count })
		limit := 5
		if len(types) < limit {
			limit = len(types)
		}
		var parts []string
		for _, tc := range types[:limit] {
			name := strings.ToLower(tc.typeName)
			name = strings.ReplaceAll(name, "_", " ")
			parts = append(parts, fmt.Sprintf("%s %s", formatCount(tc.count), name))
		}
		sec.Details = strings.Join(parts, " · ")
	}

	return sec
}

func parseLogs(resp *dqlResponse) watchSection {
	sec := watchSection{Link: "/ui/apps/dynatrace.logs/"}
	if resp == nil || len(resp.Result.Records) == 0 {
		return sec
	}

	levelCounts := make(map[string]int)
	total := 0
	for _, rec := range resp.Result.Records {
		level, _ := rec["loglevel"].(string)
		count := toInt(rec["count"])
		if level == "" {
			level = "none"
		}
		levelCounts[strings.ToLower(level)] = count
		total += count
	}
	sec.Count = total

	var parts []string
	for _, lvl := range []string{"info", "warn", "error"} {
		if c, ok := levelCounts[lvl]; ok && c > 0 {
			parts = append(parts, fmt.Sprintf("%s %s", formatCount(c), lvl))
		}
	}
	if len(parts) > 0 {
		sec.Details = strings.Join(parts, " · ")
	}
	return sec
}

func parseRequests(resp *dqlResponse) watchSection {
	sec := watchSection{Link: "/ui/apps/dynatrace.distributedtracing/explorer"}
	if resp == nil || len(resp.Result.Records) == 0 {
		return sec
	}
	rec := resp.Result.Records[0]
	success := toInt(rec["success"])
	failed := toInt(rec["failed"])
	sec.Count = success + failed
	if sec.Count > 0 {
		sec.Details = fmt.Sprintf("%s successful · %s failed", formatCount(success), formatCount(failed))
	}
	return sec
}

func parseExceptions(resp *dqlResponse) watchSection {
	sec := watchSection{Link: "/ui/apps/dynatrace.distributedtracing/exceptions"}
	if resp == nil || len(resp.Result.Records) == 0 {
		return sec
	}
	sec.Count = toInt(resp.Result.Records[0]["count"])
	return sec
}

// formatTypeBreakdown formats the top 5 entity types by count with humanized names.
func formatTypeBreakdown(types []typeCount, prefix string) string {
	if len(types) == 0 {
		return ""
	}

	sort.Slice(types, func(i, j int) bool {
		return types[i].count > types[j].count
	})

	limit := 5
	if len(types) < limit {
		limit = len(types)
	}

	var parts []string
	for _, tc := range types[:limit] {
		parts = append(parts, fmt.Sprintf("%s %s", formatCount(tc.count), humanizeTypeName(tc.typeName, prefix)))
	}
	return strings.Join(parts, ", ")
}

// humanizeTypeName converts an entity type like "AWS_LAMBDA_FUNCTION" to "lambda functions".
func humanizeTypeName(typeName, prefix string) string {
	name := strings.TrimPrefix(typeName, prefix)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, "_", " ")
	// Simple pluralization
	if !strings.HasSuffix(name, "s") {
		name += "s"
	}
	return name
}

// formatElapsed formats a duration as "Xm Ys".
func formatElapsed(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// formatCount formats an integer with comma separators.
func formatCount(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// toInt extracts an int from a DQL response value (which may be float64, json.Number, or string).
func toInt(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case json.Number:
		n, _ := val.Int64()
		return int(n)
	case int:
		return val
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 0
	}
}

// termHyperlink returns an OSC 8 clickable hyperlink for supported terminals.
// When isTTY is false, it falls back to "label (url)" plain text.
func termHyperlink(url, label string, isTTY bool) string {
	if isTTY {
		return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, label)
	}
	return fmt.Sprintf("%s (%s)", label, url)
}

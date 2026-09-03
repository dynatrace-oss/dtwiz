package installer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"golang.org/x/term"

	"github.com/dynatrace-oss/dtctl/sdk/api/query"
	"github.com/dynatrace-oss/dtctl/sdk/httpclient"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const (
	watchPollInterval = 5 * time.Second
	watchTimeout      = 10 * time.Minute
)

// watchPhase controls the DQL query strategy used for a given signal.
type watchPhase uint8

const (
	// watchPhaseProbe uses a cheap limit-1 query to detect first data arrival.
	watchPhaseProbe watchPhase = iota
	// watchPhaseMetrics switches to aggregated metrics queries once data is present.
	watchPhaseMetrics
)

// watchQueryState tracks the per-signal DQL query phase across poll cycles.
type watchQueryState struct {
	logs     watchPhase
	requests watchPhase
}

// watchInput holds a line read from stdin by the input goroutine.
type watchInput struct {
	line string
	err  error
}

// watchSection holds the display data for one section of the watch output.
type watchSection struct {
	Name    string
	Count   int
	Details string // formatted detail line (e.g. service names, type breakdown)
	Items   []watchDetail
	// Status is shown when Count == 0 but data was first seen (phase transition).
	// Used while the metrics pipeline catches up after the first data arrives.
	Status string
	// Secondary is an extra dim line rendered under the section, regardless
	// of Count. Used to surface side-signals (e.g. platform-log metrics that
	// arrive before Smartscape topology builds up).
	Secondary string
	Link      string // deep link path appended to AppsURL
}

type watchDetail struct {
	Label string
	Link  string
}

type typeCount struct {
	typeName string
	count    int
}

// dqlRecords represents a DQL query response as a list of record objects.
type dqlRecords []map[string]interface{}

// watchState holds the aggregated state across all poll cycles.
type watchState struct {
	Services      watchSection
	Hosts         watchSection
	Cloud         watchSection
	Kubernetes    watchSection
	Relationships watchSection
	Logs          watchSection
	Requests      watchSection
	Exceptions    watchSection
}

// WatchIngest polls Dynatrace for newly ingested data and renders a live
// terminal summary. It blocks until the user presses Enter or Ctrl+C.
// fromClause is injected directly into DQL queries — accepts RFC3339 timestamps
// or DQL relative expressions (e.g. "now()-1h").
func WatchIngest(envURL, pToken, fromClause string) {
	watchIngest(envURL, pToken, fromClause, nil, "", false, "")
}

// WatchIngestOtel is like WatchIngest but also shows a call to action to
// instrument the application when manualLang is the URL slug of a language
// the user selected for manual instrumentation (e.g. "go", "php").
func WatchIngestOtel(envURL, pToken, fromClause, manualLang string) {
	watchIngest(envURL, pToken, fromClause, nil, "", false, manualLang)
}

// WatchIngestCloudFromTime is like WatchIngest but calls WatchIngestCloud.
// Use this for Azure and GCP installs and updates.
func WatchIngestCloudFromTime(envURL, pToken string, startTime time.Time) {
	if startTime.IsZero() {
		return
	}
	WatchIngestCloud(envURL, pToken, startTime.UTC().Format(IngestTimeFormat))
}

// WatchIngestWithStatus is like WatchIngest but displays a background-task
// status line (e.g. a CloudFormation deployment) in the watch header.
// The caller sends status messages to statusCh; the most recent message is
// shown on every render. Passing a nil channel disables status updates.
func WatchIngestWithStatus(envURL, pToken, fromClause string, statusCh <-chan string) {
	watchIngest(envURL, pToken, fromClause, statusCh, "", false, "")
}

// WatchIngestAWS is like WatchIngestWithStatus but additionally scopes the
// cloud-platform signal queries (metrics + da-* logs) to a specific AWS
// account ID so noise from other accounts in the same tenant is filtered out.
func WatchIngestAWS(envURL, pToken, fromClause string, statusCh <-chan string, awsAccountID string) {
	watchIngest(envURL, pToken, fromClause, statusCh, awsAccountID, true, "")
}

// WatchIngestCloud is like WatchIngest but shows a "See your cloud resources
// in the Clouds app" footer instead of the QuickStart link. Use this for
// AWS, GCP, and Azure installs.
func WatchIngestCloud(envURL, pToken, fromClause string) {
	watchIngest(envURL, pToken, fromClause, nil, "", true, "")
}

// otelLangNames maps a manual-language URL slug to its display name.
var otelLangNames = map[string]string{
	"php":    "PHP",
	"cpp":    "C++",
	"dotnet": ".NET",
	"elixir": "Elixir",
	"erlang": "Erlang",
	"go":     "Go",
	"ruby":   "Ruby",
	"rust":   "Rust",
}

func watchIngest(envURL, pToken, fromClause string, statusCh <-chan string, awsAccountID string, cloudInstall bool, manualLang string) {
	if pToken == "" {
		fmt.Println("  Platform token required for watch. Set --platform-token or DT_PLATFORM_TOKEN.")
		return
	}

	appsURL := AppsURL(envURL)
	watchStart := time.Now()
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	// Colors
	highlight := color.New(color.FgMagenta, color.Bold)
	dim := color.New(color.Faint)
	green := color.New(color.FgGreen, color.Bold)
	bold := color.New(color.Bold)
	_ = green

	supportsHyperlinks := display.StdoutSupportsHyperlinks()
	linkFn := func(url, label string) string {
		return termHyperlink(url, label, supportsHyperlinks)
	}
	// Section headers and item links omit the URL when hyperlinks are not
	// supported — the raw URL breaks the watch layout without adding value.
	// The footer CTA always uses linkFn so the URL stays visible.
	sectionLinkFn := func(url, label string) string {
		if !supportsHyperlinks {
			return label
		}
		return termHyperlink(url, label, true)
	}

	var prevLines int
	var statusMsg string

	// inputCh receives one entry per newline-terminated line typed by the user.
	// Declared as nil for non-TTY — a nil channel in a select case is never chosen.
	var inputCh chan watchInput
	if isTTY {
		inputCh = make(chan watchInput, 1)
		go func() {
			reader := bufio.NewReader(os.Stdin)
			for {
				line, err := reader.ReadString('\n')
				line = strings.TrimRight(line, "\r\n")
				inputCh <- watchInput{line: line, err: err}
				if err != nil {
					return
				}
			}
		}()
	}

	qs := watchQueryState{}
	ticker := time.NewTicker(watchPollInterval)
	defer ticker.Stop()

	// Run first poll immediately, then on ticker.
	for first := true; ; first = false {
		if !first {
			select {
			case <-ticker.C:
			case inp := <-inputCh:
				if inp.err != nil {
					return
				}
				// Any Enter press during normal watching stops the loop.
				return
			}
		}

		elapsed := time.Since(watchStart).Truncate(time.Second)

		// Drain latest status message from background task (non-blocking).
		if statusCh != nil {
		drainStatus:
			for {
				select {
				case msg, ok := <-statusCh:
					if !ok {
						statusCh = nil
					} else {
						statusMsg = msg
					}
				default:
					break drainStatus
				}
			}
		}

		// After 10 minutes, prompt the user to decide whether to continue.
		if elapsed >= watchTimeout {
			if !isTTY {
				return
			}
			dim.Printf(" Continue watching? [Y/n] ")
			resp := <-inputCh
			if resp.err != nil || strings.HasPrefix(strings.ToLower(resp.line), "n") {
				return
			}
			watchStart = time.Now()
			elapsed = 0
			prevLines++ // account for the prompt line so the next render overwrites cleanly
			ticker.Reset(watchPollInterval)
			// Drain stale tick (Go docs recommend draining after Reset) and any
			// residual input buffered during the prompt (e.g. double-Enter) so
			// the next select iteration doesn't immediately stop the loop.
			select {
			case <-ticker.C:
			default:
			}
			select {
			case <-inputCh:
			default:
			}
		}

		state := pollAll(appsURL, pToken, fromClause, awsAccountID, &qs)

		var buf strings.Builder

		// Header
		highlight.Fprintf(&buf, " Watching for new data in Dynatrace... (elapsed: %s)\n", display.FormatElapsed(elapsed))
		dim.Fprintf(&buf, " Generate some load on your system to see data appear.\n")
		if statusMsg != "" {
			dim.Fprintf(&buf, " %s\n", statusMsg)
		}
		buf.WriteString("\n")

		// Sections
		renderWatchSections(&buf, state, appsURL, highlight, dim, bold, sectionLinkFn)

		// Footer
		separator := " ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"
		highlight.Fprint(&buf, separator)
		if manualLang != "" {
			if manualLang == "other" {
				otelURL := "https://dt-url.net/otel-walkthroughs"
				highlight.Fprintf(&buf, " 👉 Browse OTel instrumentation walkthroughs\n")
				fmt.Fprintf(&buf, "    %s\n", linkFn(otelURL, "→ "+otelURL))
			} else {
				langName := otelLangNames[manualLang]
				if langName == "" {
					langName = manualLang
				}
				otelURL := "https://dt-url.net/otel-" + manualLang
				highlight.Fprintf(&buf, " 👉 Instrument your %s app\n", langName)
				fmt.Fprintf(&buf, "    %s\n", linkFn(otelURL, "→ "+otelURL))
			}
		}
		if cloudInstall {
			highlight.Fprintf(&buf, " 👉 See your cloud resources in the Clouds app\n")
			fmt.Fprintf(&buf, "    %s\n", linkFn(appsURL+"/ui/apps/dynatrace.clouds/smartscape/services", "→ Open Clouds"))
		} else {
			highlight.Fprintf(&buf, " 👉 See all your data and findings in Dynatrace QuickStart\n")
			fmt.Fprintf(&buf, "    %s\n", linkFn(appsURL+"/ui/apps/dynatrace.quickstart/", "→ Open Dynatrace QuickStart"))
		}
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

func renderWatchSections(buf *strings.Builder, state watchState, appsURL string, highlight, dim, bold *color.Color, linkFn func(string, string) string) {
	renderSection(buf, "Services", state.Services, appsURL, highlight, dim, bold, linkFn)
	renderSection(buf, "Hosts", state.Hosts, appsURL, highlight, dim, bold, linkFn)
	renderSection(buf, "Kubernetes", state.Kubernetes, appsURL, highlight, dim, bold, linkFn)
	renderSection(buf, "Cloud", state.Cloud, appsURL, highlight, dim, bold, linkFn)
	renderRelationships(buf, state.Relationships, appsURL, highlight, dim, bold, linkFn)
	renderSection(buf, "Logs", state.Logs, appsURL, highlight, dim, bold, linkFn)
	renderSection(buf, "Requests", state.Requests, appsURL, highlight, dim, bold, linkFn)
	renderSection(buf, "Exceptions", state.Exceptions, appsURL, highlight, dim, bold, linkFn)
}

func renderSection(buf *strings.Builder, name string, sec watchSection, appsURL string, highlight, dim, bold *color.Color, linkFn func(string, string) string) {
	if sec.Count > 0 {
		title := name
		if sec.Link != "" {
			title = linkFn(appsURL+sec.Link, name)
		}
		highlight.Fprintf(buf, " %s", title)
		fmt.Fprintf(buf, " (%s)\n", display.FormatCount(sec.Count))
		if details := renderSectionDetails(sec, appsURL, linkFn); details != "" {
			fmt.Fprintf(buf, "   %s\n", details)
		}
	} else if sec.Status != "" {
		title := name
		if sec.Link != "" {
			title = linkFn(appsURL+sec.Link, name)
		}
		highlight.Fprintf(buf, " %s\n", title)
		dim.Fprintf(buf, "   %s\n", sec.Status)
	} else {
		highlight.Fprintf(buf, " %s\n", name)
		dim.Fprintf(buf, "   waiting...\n")
	}
	if sec.Secondary != "" {
		dim.Fprintf(buf, "   %s\n", sec.Secondary)
	}
	buf.WriteString("\n")
}

func renderSectionDetails(sec watchSection, appsURL string, linkFn func(string, string) string) string {
	parts := make([]string, 0, len(sec.Items)+1)
	for _, item := range sec.Items {
		if item.Label == "" {
			continue
		}
		if item.Link != "" {
			parts = append(parts, linkFn(appsURL+item.Link, item.Label))
			continue
		}
		parts = append(parts, item.Label)
	}
	if sec.Details != "" {
		parts = append(parts, sec.Details)
	}
	return strings.Join(parts, ", ")
}

func renderRelationships(buf *strings.Builder, sec watchSection, appsURL string, highlight, dim, bold *color.Color, linkFn func(string, string) string) {
	if sec.Count > 0 {
		title := "Relationships"
		if sec.Link != "" {
			title = linkFn(appsURL+sec.Link, "Relationships")
		}
		highlight.Fprintf(buf, " %s", title)
		fmt.Fprintf(buf, " (%s)\n", display.FormatCount(sec.Count))
		if sec.Details != "" {
			fmt.Fprintf(buf, "   %s\n", sec.Details)
		}
	} else {
		highlight.Fprintf(buf, " Relationships\n")
		dim.Fprintf(buf, "   waiting...\n")
	}
	buf.WriteString("\n")
}

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

// dqlEscapeString escapes a value for safe interpolation into a DQL
// double-quoted string literal. Backslashes and double quotes are the only
// metacharacters inside `"..."` literals.
var dqlStringEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

func dqlEscapeString(s string) string { return dqlStringEscaper.Replace(s) }

// pollAll executes all DQL queries in parallel and returns the aggregated state.
// qs tracks per-signal query phases and is updated in-place as phases advance.
func pollAll(appsURL, token, fromClause, awsAccountID string, qs *watchQueryState) watchState {
	var state watchState

	type result struct {
		name string
		data dqlRecords
	}

	from := dqlFromLiteral(fromClause)

	queries := map[string]string{
		"services":      fmt.Sprintf(`smartscapeNodes SERVICE, from:%s | fields name | limit 100`, from),
		"hosts":         fmt.Sprintf(`smartscapeNodes "*", from:%s | filter type == "HOST" or type == "OTEL_HOST" | fields id, name, type | sort name asc | limit 100`, from),
		"nodes":         fmt.Sprintf(`smartscapeNodes "*", from:%s | summarize count=count(), by:{type} | limit 200`, from),
		"relationships": fmt.Sprintf(`smartscapeEdges "*", from:%s | summarize count=count(), by:{type}`, from),
		"exceptions":    fmt.Sprintf(`fetch spans, from:%s | expand events = span.events | filter events[type] == "exception" | summarize count=count()`, from),
	}

	// Logs: probe phase uses a cheap limit-1 fetch to detect first arrival;
	// metrics phase runs the full summarize — but only after we know logs exist,
	// so we never pay the full-scan cost on an empty dataset.
	switch qs.logs {
	case watchPhaseProbe:
		queries["logs"] = fmt.Sprintf(`fetch logs, from:%s | limit 1`, from)
	case watchPhaseMetrics:
		queries["logs"] = fmt.Sprintf(`fetch logs, from:%s | summarize count=count(), by:{loglevel}`, from)
	}

	// Requests: probe phase uses a cheap limit-1 span fetch to detect first
	// arrival; metrics phase runs the full summarize — but only after we know
	// spans exist, so we never pay the full-scan cost on an empty dataset.
	switch qs.requests {
	case watchPhaseProbe:
		queries["requests"] = fmt.Sprintf(`fetch spans, from:%s | filter request.is_root_span == true | limit 1`, from)
	case watchPhaseMetrics:
		queries["requests"] = fmt.Sprintf(`fetch spans, from:%s | filter request.is_root_span == true | summarize failed=countIf(request.is_failed == true), success=countIf(request.is_failed != true)`, from)
	}

	// Cloud-platform signals (metric registrations + da-* integration logs) are
	// only useful when scoped to a specific AWS account, and they would otherwise
	// add two extra DQL queries to every poll for non-AWS installs. Gate them on
	// an explicit account ID so generic `dtwiz watch` is unaffected.
	if awsAccountID != "" {
		cloudAccountFilter := fmt.Sprintf(`| filter aws.account.id == "%s" `, dqlEscapeString(awsAccountID))
		// Metrics is metadata-only (count of registered AWS metric series) so it
		// intentionally uses the default timeframe; logs are data so they honour `from`.
		queries["cloud_metrics"] = fmt.Sprintf(`metrics | filter startsWith(metric.key, "cloud.aws.") %s| summarize count=count(), by:{aws.resource.type} | limit 50`, cloudAccountFilter)
		queries["cloud_logs"] = fmt.Sprintf(`fetch logs, from:%s | filter startsWith(dt.openpipeline.source, "da-") %s| summarize count=count(), by:{aws.resource.type} | limit 50`, from, cloudAccountFilter)
	}

	ch := make(chan result, len(queries))
	for name, dql := range queries {
		go func(n, q string) {
			ch <- result{name: n, data: executeDQL(appsURL, token, q)}
		}(name, dql)
	}

	results := make(map[string]dqlRecords, len(queries))
	for range queries {
		r := <-ch
		results[r.name] = r.data
	}

	// Services
	state.Services = parseServices(results["services"])
	// Hosts
	state.Hosts = parseHosts(results["hosts"])
	// Cloud + Kubernetes from nodes
	state.Cloud, state.Kubernetes = parseNodes(results["nodes"])
	// Cloud platform signals (metrics + logs from da-* integrations)
	state.Cloud.Secondary = parseCloudPlatformSignals(results["cloud_metrics"], results["cloud_logs"])
	// Relationships
	state.Relationships = parseRelationships(results["relationships"])

	// Logs: probe detects first arrival and advances to metrics phase;
	// metrics phase shows the level breakdown once the pipeline catches up.
	switch qs.logs {
	case watchPhaseProbe:
		if len(results["logs"]) > 0 {
			qs.logs = watchPhaseMetrics
			state.Logs = watchSection{Link: "/ui/apps/dynatrace.logs/", Status: "Logs ingested"}
		}
	case watchPhaseMetrics:
		state.Logs = parseLogs(results["logs"])
		if state.Logs.Count == 0 {
			// Metrics pipeline hasn't aggregated yet; hold the ingested status.
			state.Logs.Status = "Logs ingested"
		}
	}

	// Requests: probe detects first span and advances to metrics phase;
	// metrics phase shows success/failed counts once the pipeline catches up.
	switch qs.requests {
	case watchPhaseProbe:
		if len(results["requests"]) > 0 {
			qs.requests = watchPhaseMetrics
			state.Requests = watchSection{Link: "/ui/apps/dynatrace.distributedtracing/explorer", Status: "Requests ingested"}
		}
	case watchPhaseMetrics:
		state.Requests = parseRequests(results["requests"])
		if state.Requests.Count == 0 {
			// Metrics pipeline hasn't aggregated yet; hold the ingested status.
			state.Requests.Status = "Requests ingested"
		}
	}

	// Exceptions
	state.Exceptions = parseExceptions(results["exceptions"])

	return state
}

func executeDQL(appsURL, token, dql string) dqlRecords {
	c, err := httpclient.New(appsURL, httpclient.WithToken(token))
	if err != nil {
		logger.Debug("watch DQL client error", "err", err)
		return nil
	}
	h := query.NewHandler(c)
	resp, err := h.ExecuteAndPoll(context.Background(), query.ExecuteRequest{
		Query:                      dql,
		RequestTimeoutMilliseconds: 10000,
		MaxResultRecords:           200,
	}, nil)
	if err != nil {
		logger.Debug("watch DQL request failed", "err", err)
		return nil
	}
	return resp.GetRecords()
}

func parseServices(records dqlRecords) watchSection {
	sec := watchSection{Link: "/ui/apps/dynatrace.services/explorer-new/services-new"}
	if len(records) == 0 {
		return sec
	}

	var names []string
	for _, rec := range records {
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

func parseHosts(records dqlRecords) watchSection {
	sec := watchSection{Link: "/ui/apps/dynatrace.infraops/smartscape/Compute/Hosts?perspective=Health&sort=healthIndicators%3Adescending"}
	if len(records) == 0 {
		return sec
	}

	for _, rec := range records {
		id, _ := rec["id"].(string)
		name, _ := rec["name"].(string)
		typeName, _ := rec["type"].(string)
		link := hostDetailLink(typeName, id)
		if id == "" || name == "" || link == "" {
			continue
		}

		sec.Count++
		if len(sec.Items) < 5 {
			sec.Items = append(sec.Items, watchDetail{Label: name, Link: link})
		}
	}

	if sec.Count > len(sec.Items) {
		sec.Details = fmt.Sprintf("+%d more", sec.Count-len(sec.Items))
	}
	return sec
}

func hostDetailLink(typeName, id string) string {
	entityID := url.QueryEscape(id)
	switch typeName {
	case "HOST":
		return "/ui/apps/dynatrace.infraops/smartscape/Compute/Hosts?perspective=Health&sort=healthIndicators%3Adescending&fullPageId=" + entityID
	case "OTEL_HOST":
		return "/ui/apps/dynatrace.infraops/smartscape/Compute/com.dynatrace.extension.opentelemetry/OTEL_HOST-inventory?perspective=health&sort=healthIndicators%3Adescending&detailsId=" + entityID + "&sidebarOpen=false"
	default:
		return ""
	}
}

func parseNodes(records dqlRecords) (cloud, k8s watchSection) {
	cloud = watchSection{Link: "/ui/apps/dynatrace.clouds/smartscape/services"}
	k8s = watchSection{Link: "/ui/apps/dynatrace.kubernetes/smartscape/K8S_CLUSTER"}

	var awsTypes, azureTypes, gcpTypes, k8sTypes []typeCount

	for _, rec := range records {
		typeName, _ := rec["type"].(string)
		count := toInt(rec["count"])
		if count == 0 {
			continue
		}

		switch {
		case strings.HasPrefix(typeName, "AWS_"):
			awsTypes = append(awsTypes, typeCount{typeName, count})
			cloud.Count += count
		case strings.HasPrefix(typeName, "AZURE_"):
			azureTypes = append(azureTypes, typeCount{typeName, count})
			cloud.Count += count
		case strings.HasPrefix(typeName, "GCP_"):
			gcpTypes = append(gcpTypes, typeCount{typeName, count})
			cloud.Count += count
		case strings.HasPrefix(typeName, "K8S_") || typeName == "CONTAINER":
			k8sTypes = append(k8sTypes, typeCount{typeName, count})
			k8s.Count += count
		}
	}

	var cloudDetailParts []string
	if s := formatTypeBreakdown(awsTypes, "AWS_"); s != "" {
		cloudDetailParts = append(cloudDetailParts, s)
	}
	if s := formatTypeBreakdown(azureTypes, "AZURE_MICROSOFT_"); s != "" {
		cloudDetailParts = append(cloudDetailParts, s)
	}
	if s := formatTypeBreakdown(gcpTypes, "GCP_"); s != "" {
		cloudDetailParts = append(cloudDetailParts, s)
	}
	cloud.Details = strings.Join(cloudDetailParts, ", ")
	k8s.Details = formatTypeBreakdown(k8sTypes, "K8S_")

	return
}

func parseRelationships(records dqlRecords) watchSection {
	sec := watchSection{Link: "/ui/apps/dynatrace.smartscape/view/dynatrace.smartscape.smartscape-on-grail"}
	if len(records) == 0 {
		return sec
	}

	var types []typeCount
	for _, rec := range records {
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
			parts = append(parts, fmt.Sprintf("%s %s", display.FormatCount(tc.count), name))
		}
		sec.Details = strings.Join(parts, " · ")
	}

	return sec
}

func parseLogs(records dqlRecords) watchSection {
	sec := watchSection{Link: "/ui/apps/dynatrace.logs/"}
	if len(records) == 0 {
		return sec
	}

	levelCounts := make(map[string]int)
	total := 0
	for _, rec := range records {
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
			parts = append(parts, fmt.Sprintf("%s %s", display.FormatCount(c), lvl))
		}
	}
	if len(parts) > 0 {
		sec.Details = strings.Join(parts, " · ")
	}
	return sec
}

func parseRequests(records dqlRecords) watchSection {
	sec := watchSection{Link: "/ui/apps/dynatrace.distributedtracing/explorer"}
	if len(records) == 0 {
		return sec
	}
	rec := records[0]
	success := toInt(rec["success"])
	failed := toInt(rec["failed"])
	sec.Count = success + failed
	if sec.Count > 0 {
		sec.Details = fmt.Sprintf("%s successful · %s failed", display.FormatCount(success), display.FormatCount(failed))
	}
	return sec
}

func parseExceptions(records dqlRecords) watchSection {
	sec := watchSection{Link: "/ui/apps/dynatrace.distributedtracing/exceptions"}
	if len(records) == 0 {
		return sec
	}
	sec.Count = toInt(records[0]["count"])
	return sec
}

// parseCloudPlatformSignals summarises AWS data-acquisition signals (metric
// registrations and integration logs grouped by `aws.resource.type`) into a
// single dim line that lists the actual resource types detected, e.g.:
//
//	cloud signals (5 types): Lambda::Function · ApiGatewayV2::Api · EC2::Instance · RDS::DBInstance · EKS::Cluster
//
// Records with null/empty `aws.resource.type` are ignored — they are noise
// (uncategorised metrics).
func parseCloudPlatformSignals(metrics, logs dqlRecords) string {
	combined := make(map[string]int)
	collectResourceTypes(metrics, combined)
	collectResourceTypes(logs, combined)

	if len(combined) == 0 {
		return ""
	}

	types := make([]typeCount, 0, len(combined))
	for name, c := range combined {
		types = append(types, typeCount{name, c})
	}
	// Tie-break on typeName so the rendered order stays stable between polls
	// (map iteration order would otherwise make equal-count entries swap).
	sort.Slice(types, func(i, j int) bool {
		if types[i].count != types[j].count {
			return types[i].count > types[j].count
		}
		return types[i].typeName < types[j].typeName
	})

	limit := 5
	if len(types) < limit {
		limit = len(types)
	}
	names := make([]string, 0, limit)
	for _, tc := range types[:limit] {
		names = append(names, shortResourceType(tc.typeName))
	}

	suffix := ""
	if len(types) > limit {
		suffix = fmt.Sprintf(" +%d more", len(types)-limit)
	}
	return fmt.Sprintf("cloud signals (%d type%s): %s%s", len(types), plural(len(types)), strings.Join(names, " · "), suffix)
}

// collectResourceTypes accumulates non-null `aws.resource.type` counts into dst.
func collectResourceTypes(records dqlRecords, dst map[string]int) {
	for _, rec := range records {
		rt, _ := rec["aws.resource.type"].(string)
		if rt == "" {
			continue
		}
		c := toInt(rec["count"])
		if c == 0 {
			continue
		}
		dst[rt] += c
	}
}

// shortResourceType strips the redundant "AWS::" prefix so labels stay compact:
// "AWS::Lambda::Function" -> "Lambda::Function".
func shortResourceType(rt string) string {
	return strings.TrimPrefix(rt, "AWS::")
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
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
		parts = append(parts, fmt.Sprintf("%s %s", display.FormatCount(tc.count), humanizeTypeName(tc.typeName, prefix)))
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

package selfmonitoring

import (
	"fmt"
	"strings"

	"github.com/dynatrace-oss/dtwiz/pkg/version"
)

const (
	propExecID        = "e"
	propCmd           = "c"
	propStep          = "st"
	propSub           = "s"
	propErr           = "er"
	propType          = "t"
	propK8sDistro     = "kd"
	propCloudProvider = "cp"
	propOpt           = "opt"
)

// buildUserAgent encodes operation identity into User-Agent (64-char HAProxy capture limit).
// Format: dtwiz/<version>[;c=<cmd>];st=<step>[;s=<sub>][;er=<err>][;t=<type>]
// c= is omitted when Cmd is empty. ExecID, mode, and OS go into Tab-Id via buildTabID.
// Command, subcommand, and error are abbreviated to fit the limit; the event body carries
// the full names.
func buildUserAgent(p EventParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "dtwiz/%s", version.Version)
	pairs := []struct{ k, v string }{
		{propCmd, shortCmd(p.Cmd)},
		{propStep, p.StepID},
		{propSub, shortSub(p.Sub)},
		{propErr, shortErr(p.Err)},
		{propType, p.Type},
		{propCloudProvider, shortCloudProvider(p.CloudProvider)},
		{propOpt, shortSub(p.Opt)},
	}
	// kd= is only included for analyze events;
	if p.StepID == StepAnalyze {
		pairs = append(pairs, struct{ k, v string }{propK8sDistro, shortDistro(p.K8sDistro)})
	}
	for _, kv := range pairs {
		if kv.v != "" {
			fmt.Fprintf(&b, ";%s=%s", kv.k, kv.v)
		}
	}
	return b.String()
}

// buildTabID encodes execution context into Tab-Id (16-char HAProxy capture limit).
// Format: <execid>;m=<mode>;o=<os> — execid is positional (always 3 hex chars), mode and os are 3 chars each.
// Worst case: "3ab;m=deb;o=win" = 15 chars.
func buildTabID(p EventParams) string {
	return execID + ";m=" + shortMode(p.Mode) + ";o=" + resolveOS()
}

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
	propFeatures      = "f"
	propDurationS     = "d"
)

// buildUserAgent encodes operation identity into User-Agent (64-char HAProxy capture limit).
// Format: dtwiz/<version>;c=<cmd>;st=<step>[;s=<sub>][;er=<err>] followed by step-specific fields:
//
//	snapshot:                         ;t=<type>
//	install:                          ;f=<feature outcomes>;d=<work time seconds>
//	analyze:                          ;cp=<cloud providers>;kd=<k8s distro>
//	recommendations presented/selected: ;opt=<option>
//
// Empty values are omitted, so c= is dropped when Cmd is empty. Command, subcommand, option,
// error, cloud provider, and distro are abbreviated to fit the limit.
func buildUserAgent(p EventParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "dtwiz/%s", version.Version)
	add := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, ";%s=%s", k, v)
		}
	}

	add(propCmd, shortCmd(p.Cmd))
	add(propStep, p.StepID)
	add(propSub, shortSub(p.Sub))
	add(propErr, shortErr(p.Err))

	switch p.StepID {
	case StepSnapshot:
		add(propType, p.Type)
	case StepInstall:
		add(propFeatures, p.Features)
		add(propDurationS, p.DurationS)
	case StepAnalyze:
		add(propCloudProvider, shortCloudProvider(p.CloudProvider))
		add(propK8sDistro, shortDistro(p.K8sDistro))
	case StepRecommendationsPresented, StepRecommendationsSelected:
		add(propOpt, shortOpt(p.Opt))
	}
	return b.String()
}

// buildTabID encodes execution context into Tab-Id (16-char HAProxy capture limit).
// Format: <execid>;m=<mode>;o=<os> — execid is positional (always 3 hex chars), mode and os are 3 chars each.
// Worst case: "3ab;m=deb;o=win" = 15 chars.
func buildTabID(p EventParams) string {
	return execID + ";m=" + shortMode(p.Mode) + ";o=" + resolveOS()
}

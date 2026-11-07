package otel

import (
	"context"
	"fmt"
	"strings"
)

// goPreflight checks that a service's package resolves on its own, with
// workspace mode off, which is how it will be built. A module that imports a
// sibling only through go.work fails here (missing go.sum entry, unknown
// module) and would fail the build with either method. `-deps` is needed:
// plain `go list` does not load imports and passes for such a module.
func goPreflight(ctx context.Context, svc GoService, moduleDir string) error {
	out, err := goBuildRunner(ctx, moduleDir, goBuildEnv(), "go", "list", "-deps", goPackageArg(svc))
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	detail := lastLines(out, 3)
	if detail == "" {
		detail = err.Error()
	}
	return fmt.Errorf("%s", strings.ReplaceAll(detail, "\n", "; "))
}

// goPackageArg is the package pattern of a service relative to its module.
func goPackageArg(svc GoService) string {
	if svc.RelDir == "." || svc.RelDir == "" {
		return "."
	}
	return "./" + svc.RelDir
}

// runGoPreflight marks services of a workspace project manual when they cannot
// be built in isolation, and tells the user why.
func (p *GoInstrumentationPlan) runGoPreflight(ctx context.Context) {
	for _, s := range p.Services {
		if ctx.Err() != nil {
			return
		}
		if s.Method == goMethodManual {
			continue
		}
		if err := goPreflight(ctx, s.GoService, s.moduleDir(p.Project.Path)); err != nil {
			s.Method = goMethodManual
			s.Reason = fmt.Sprintf("its module cannot be built on its own without go.work (for example it depends on a workspace sibling): %v", err)
		}
	}
}

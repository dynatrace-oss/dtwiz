package grail

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func checkDQLStatus(code int, op string, body []byte) error {
	if code != http.StatusOK && code != http.StatusAccepted {
		return fmt.Errorf("DQL %s returned HTTP %d: %s", op, code, body)
	}
	return nil
}

func sleepOrCancel(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

func tracesByServiceQuery(serviceName string) string {
	return fmt.Sprintf(
		`smartscapeNodes "SERVICE", from: -30m, to: now() | filter name == %q`,
		serviceName,
	)
}

func hostByNameQuery(hostName string) string {
	return fmt.Sprintf(
		`smartscapeNodes "HOST", from: -30m, to: now() | filter name == %q`,
		hostName,
	)
}

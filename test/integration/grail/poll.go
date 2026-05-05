package grail

import (
	"context"
	"fmt"
	"log"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

func pollDQL(ctx context.Context, platform *client.PlatformClient, requestToken string) ([]TraceRecord, error) {
	for i := 0; i < dqlPollMaxRetries; i++ {
		log.Printf("DQL poll: attempt %d/%d for token %q", i+1, dqlPollMaxRetries, requestToken)
		var resp grailResponse
		raw, err := platform.HTTP().R().
			SetContext(ctx).
			SetQueryParam("request-token", requestToken).
			SetResult(&resp).
			Get(grailPollPath)
		if err != nil {
			return nil, fmt.Errorf("DQL poll: %w", err)
		}

		log.Printf("DQL poll: got HTTP %d (state=%s)", raw.StatusCode(), resp.State)
		if err := checkDQLStatus(raw.StatusCode(), "poll", raw.Body()); err != nil {
			return nil, err
		}
		switch resp.State {
		case "SUCCEEDED":
			log.Printf("DQL poll: query completed after %d attempt(s), records=%d", i+1, len(resp.Result.Records))
			return resp.Result.Records, nil
		case "RUNNING":
			log.Printf("DQL poll: query still running, waiting %s before next attempt", dqlPollInterval)
			if i < dqlPollMaxRetries-1 {
				if err := sleepOrCancel(ctx, dqlPollInterval); err != nil {
					return nil, fmt.Errorf("DQL poll: %w", err)
				}
			}
		default:
			return nil, fmt.Errorf("DQL poll: unexpected state %q", resp.State)
		}
	}
	return nil, fmt.Errorf("DQL poll: exceeded %d retries for token %q", dqlPollMaxRetries, requestToken)
}

package grail

import (
	"context"
	"fmt"
	"log"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

func executeDQL(ctx context.Context, platform *client.PlatformClient, dql string) ([]TraceRecord, error) {
	payload := map[string]interface{}{
		"query":                      dql,
		"requestTimeoutMilliseconds": 10000,
		"maxResultRecords":           200,
	}

	log.Printf("DQL execute: posting query to %s", grailExecutePath)
	var resp grailResponse
	raw, err := platform.HTTP().R().
		SetContext(ctx).
		SetBody(payload).
		SetResult(&resp).
		Post(grailExecutePath)
	if err != nil {
		return nil, err
	}

	log.Printf("DQL execute: got HTTP %d (state=%s)", raw.StatusCode(), resp.State)
	if err := checkDQLStatus(raw.StatusCode(), "execute", raw.Body()); err != nil {
		return nil, err
	}
	switch resp.State {
	case "SUCCEEDED":
		log.Printf("DQL execute: query completed, records=%d", len(resp.Result.Records))
		return resp.Result.Records, nil
	case "RUNNING":
		if resp.RequestToken == "" {
			return nil, fmt.Errorf("DQL execute: RUNNING state but no requestToken in response")
		}
		log.Printf("DQL execute: query running — starting poll")
		return pollDQL(ctx, platform, resp.RequestToken)
	default:
		return nil, fmt.Errorf("DQL execute: unexpected state %q", resp.State)
	}
}

package grail

import (
	"context"
	"testing"

	"github.com/dynatrace-oss/dtwiz/pkg/client"
)

// RequireKubernetesCluster calls WaitForKubernetesCluster and fatals if it errors or returns no records.
func RequireKubernetesCluster(t *testing.T, c *client.Client, clusterName string, opts ...PollOption) []TraceRecord {
	t.Helper()
	clusters, err := WaitForKubernetesCluster(context.Background(), c, clusterName, opts...)
	if err != nil {
		t.Fatalf("WaitForKubernetesCluster: %v", err)
	}
	if len(clusters) == 0 {
		t.Fatalf("expected Kubernetes cluster %q to appear in topology, got none", clusterName)
	}
	return clusters
}

// WaitForKubernetesCluster polls the DQL endpoint via PlatformClient until a
// K8S_CLUSTER entity named clusterName appears in the Smartscape topology or
// the timeout is exceeded. It is used to confirm that the Dynatrace Operator
// actually reported cluster topology, not just that its pods became ready.
func WaitForKubernetesCluster(ctx context.Context, c *client.Client, clusterName string, options ...PollOption) ([]TraceRecord, error) {
	return waitForRecords(ctx, c, kubernetesClusterByNameQuery(clusterName), "Kubernetes cluster "+clusterName+" in topology", options...)
}

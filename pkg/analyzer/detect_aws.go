package analyzer

import (
	"strconv"
	"strings"
	"sync"
)

// detectAWS checks for an AWS CLI, verifies credentials, and probes common services.
func detectAWS() *AWSInfo {
	info := &AWSInfo{}

	ok, out := runCmd("aws", "sts", "get-caller-identity", "--output", "text", "--query", "Account")
	if !ok {
		return info
	}
	info.Available = true
	info.AccountID = strings.TrimSpace(out)

	_, region := runCmd("aws", "configure", "get", "region")
	info.Region = strings.TrimSpace(region)

	info.Services = detectAWSServices()
	return info
}

type awsServiceProbe struct {
	name string
	cmd  []string
	// countFn extracts a count from the command output; returns 0 if nothing found.
	countFn func(out string) int
}

func parseIntFirst(out string) int {
	f := strings.Fields(strings.TrimSpace(out))
	if len(f) == 0 {
		return 0
	}
	n, err := strconv.Atoi(f[0])
	if err != nil {
		return 0
	}
	return n
}

var awsProbes = []awsServiceProbe{
	{
		name: "EC2",
		cmd:  []string{"aws", "ec2", "describe-instances", "--filters", "Name=instance-state-name,Values=running", "--query", "length(Reservations[].Instances[])", "--output", "text", "--cli-read-timeout", "20"},
		countFn: parseIntFirst,
	},
	{
		name: "EKS",
		cmd:  []string{"aws", "eks", "list-clusters", "--query", "length(clusters)", "--output", "text", "--cli-read-timeout", "20"},
		countFn: parseIntFirst,
	},
	{
		name: "ECS",
		cmd:  []string{"aws", "ecs", "list-clusters", "--query", "length(clusterArns)", "--output", "text", "--cli-read-timeout", "20"},
		countFn: parseIntFirst,
	},
	{
		name: "Lambda",
		cmd:  []string{"aws", "lambda", "list-functions", "--query", "length(Functions)", "--output", "text", "--cli-read-timeout", "20"},
		countFn: parseIntFirst,
	},
	{
		name: "RDS",
		cmd:  []string{"aws", "rds", "describe-db-instances", "--query", "length(DBInstances)", "--output", "text", "--cli-read-timeout", "20"},
		countFn: parseIntFirst,
	},
	{
		name: "S3",
		cmd:  []string{"aws", "s3api", "list-buckets", "--query", "length(Buckets)", "--output", "text", "--cli-read-timeout", "20"},
		countFn: parseIntFirst,
	},
}

func detectAWSServices() []AWSService {
	type result struct {
		svc AWSService
		valid bool
	}
	results := make([]result, len(awsProbes))
	var wg sync.WaitGroup
	for i, probe := range awsProbes {
		wg.Add(1)
		go func(i int, probe awsServiceProbe) {
			defer wg.Done()
			ok, out := runCmd(probe.cmd[0], probe.cmd[1:]...)
			if !ok {
				return
			}
			count := probe.countFn(out)
			if count > 0 {
				results[i] = result{svc: AWSService{Name: probe.name, Count: count}, valid: true}
			}
		}(i, probe)
	}
	wg.Wait()
	var services []AWSService
	for _, r := range results {
		if r.valid {
			services = append(services, r.svc)
		}
	}
	return services
}

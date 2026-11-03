package main

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestHostCountIsNotTruncated(t *testing.T) {
	hosts := make([]string, 0, 125)
	for i := 0; i < 125; i++ {
		hosts = append(hosts, strconv.Quote(fmt.Sprintf("route-%03d.example.test", i)))
	}
	config := `{"apps":{"http":{"servers":{"edge":{"routes":[{"match":[{"host":[` + strings.Join(hosts, ",") + `]}]}]}}}}}`
	result, err := summarize(strings.NewReader(config))
	if err != nil {
		t.Fatal(err)
	}
	if result.HostCount == nil || *result.HostCount != 125 {
		t.Fatalf("expected full host count, got %+v", result.HostCount)
	}
	if len(result.Hosts) != 100 {
		t.Fatalf("expected bounded list, got %d", len(result.Hosts))
	}
	if result.Hosts[0] != "route-000.example.test" || result.Hosts[99] != "route-099.example.test" {
		t.Fatalf("unexpected sorted hostnames: first=%q last=%q", result.Hosts[0], result.Hosts[99])
	}
	if result.Routes == nil || *result.Routes != 1 {
		t.Fatalf("expected one top-level route, got %+v", result.Routes)
	}
}

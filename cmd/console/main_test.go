package main

import "testing"

func TestConsoleBindSafety(t *testing.T) {
 for _, value := range []string{"0.0.0.0:7689", ":7689", "localhost:7689"} {
  if validateListen(value) == nil { t.Errorf("public or ambiguous bind allowed: %s", value) }
 }
 for _, value := range []string{"127.0.0.1:7689", "[::1]:7689"} {
  if err := validateListen(value); err != nil { t.Errorf("loopback rejected: %v", err) }
 }
}

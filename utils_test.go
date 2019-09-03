package main

import (
	"testing"
)

func TestGetRandomString(t *testing.T) {
	sLen := 500
	s := getRandomString(sLen)
	if len(s) != sLen {
		t.Fatalf("%d %s %d", len(s), "<", sLen)
	}
}

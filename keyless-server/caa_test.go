package main

import (
	"testing"
)

func TestConvertTXTtoCAA(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{
			"empty",
			"\x00\x10\x00\x01\x00\x00\x00\x00\x00\x0c\x0b0 issue \";\"",
			"\x01\x01\x00\x01\x00\x00\x00\x00\x00\x08\x00\x05issue;",
		},
		{
			"empty",
			"\x00\x10\x00\x01\x00\x00\x00\x00\x00\x1e\x1d0 issuewild \"letsencrypt.org\"",
			"\x01\x01\x00\x01\x00\x00\x00\x00\x00\x1a\x00\x09issuewildletsencrypt.org",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(convertTXTtoCAA([]byte(tt.in)))
			if got != tt.out {
				t.Errorf("got %q, wanted %q", got, tt.out)
			}
		})
	}
}

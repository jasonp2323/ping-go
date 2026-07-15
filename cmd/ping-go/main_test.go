package main

import (
	"reflect"
	"testing"
)

func TestParseHosts(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"single", "google.com", []string{"google.com"}},
		{"multiple", "google.com,8.8.8.8", []string{"google.com", "8.8.8.8"}},
		{"trims whitespace", " google.com , 8.8.8.8 ", []string{"google.com", "8.8.8.8"}},
		{"drops empty entries", "google.com,,,8.8.8.8,", []string{"google.com", "8.8.8.8"}},
		{"empty string", "", nil},
		{"only separators", ",, ,", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseHosts(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseHosts(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

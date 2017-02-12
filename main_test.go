package main

import (
	"fmt"
	"testing"
)

func TestNormalizeAddr(t *testing.T) {
	cases := []struct {
		name string
		i    string
		e    string
	}{
		{
			"empty",
			"",
			"",
		},
		{
			"scheme",
			"www.example.com",
			"https://www.example.com/",
		},
		{
			"trailing-slash",
			"https://www.example.com/foo",
			"https://www.example.com/foo/",
		},
		{
			"trailing-slash-many",
			"https://www.example.com/foo///////",
			"https://www.example.com/foo/",
		},
		{
			"no-overwrite-scheme",
			"ftp://foo.com/",
			"ftp://foo.com/",
		},
		{
			"port",
			"www.example.com:8200",
			"https://www.example.com:8200/",
		},
		{
			"port-scheme",
			"http://www.example.com:8200",
			"http://www.example.com:8200/",
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			r := normalizeAddr(tc.i)
			if r != tc.e {
				t.Errorf("expected %q to be %q", r, tc.e)
			}
		})
	}
}

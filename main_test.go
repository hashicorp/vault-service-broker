package main

import (
	"fmt"
	"reflect"
	"testing"

	"code.cloudfoundry.org/lager"
)

func TestParseLogLevel(t *testing.T) {
	cases := []struct {
		name  string
		level string
		exp   lager.LogLevel
		err   bool
	}{
		{
			"debug",
			"debug",
			lager.DEBUG,
			false,
		},
		{
			"info",
			"info",
			lager.INFO,
			false,
		},
		{
			"error",
			"error",
			lager.ERROR,
			false,
		},
		{
			"fatal",
			"fatal",
			lager.FATAL,
			false,
		},
		{
			"banana",
			"banana",
			0,
			true,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d_%s", i, tc.name), func(t *testing.T) {
			act, err := parseLogLevel(tc.level)
			if (err != nil) != tc.err {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(tc.exp, act) {
				t.Errorf("\nexp: %#v\nact: %#v", tc.exp, act)
			}
		})
	}
}

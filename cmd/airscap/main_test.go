package main

import "testing"

func TestEnvBool(t *testing.T) {
	const key = "AIRSCAP_ROTATE_180"
	for _, value := range []string{"1", "t", "T", "TRUE", "true", "True"} {
		t.Setenv(key, value)
		if !envBool(key, false) {
			t.Errorf("%q not accepted as true", value)
		}
	}
	for _, value := range []string{"0", "f", "F", "FALSE", "false", "False"} {
		t.Setenv(key, value)
		if envBool(key, true) {
			t.Errorf("%q not accepted as false", value)
		}
	}
	for _, value := range []string{"", "invalid"} {
		t.Setenv(key, value)
		for _, fallback := range []bool{false, true} {
			if got := envBool(key, fallback); got != fallback {
				t.Errorf("%q: got %v want fallback %v", value, got, fallback)
			}
		}
	}
}

package server

import "testing"

func TestIsSerialStreamPath(t *testing.T) {
	cases := map[string]bool{
		"/vms/11111111-1111-1111-1111-111111111111/serial-stream": true,
		"/vms/abc/serial-stream":                                  true,
		"/vms/abc/screen-stream":                                  false,
		"/vms/abc/serial-console":                                 false,
		"/vms/serial-stream":                                      false,
		"/vms/abc/serial-stream/extra":                            false,
		"/serial-stream":                                          false,
	}
	for path, want := range cases {
		if got := isSerialStreamPath(path); got != want {
			t.Errorf("isSerialStreamPath(%q) = %v, want %v", path, got, want)
		}
	}
}

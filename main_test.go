package hdl

import (
	"testing"
)

func TestHDLHandler(t *testing.T) {
	tests := map[string]string{
		"/hdl/abc.flv": "abc", "/hdl/abc": "abc", "/abc": "abc", "/abc.flv": "abc",
	}
	for name, result := range tests {
		t.Run(name, func(t *testing.T) {
			parts := streamPathReg.FindStringSubmatch(name)
			stringPath := parts[3]
			if stringPath == "" {
				stringPath = parts[5]
			}
			if stringPath != result {
				t.Fail()
			}
		})
	}
}

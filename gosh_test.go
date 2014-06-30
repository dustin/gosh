package main

import (
	"os/exec"
	"testing"
	"time"
)

func TestRunCmd(t *testing.T) {
	tests := []struct {
		cmd         string
		args        []string
		shouldError bool
	}{
		{"/usr/bin/false", nil, true},
		{"/usr/bin/true", nil, false},
		{"/bin/sleep", []string{"900"}, true},
	}

	*timeout = time.Second

	for _, test := range tests {
		cmd := exec.Command(test.cmd, test.args...)
		err := runCmd(cmd)
		if (err != nil) != test.shouldError {
			t.Errorf("%v(%v): Error expectation was %v. error was %v",
				test.cmd, test.args, test.shouldError, err)
		}
	}
}

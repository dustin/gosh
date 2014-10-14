package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"
)

const uninterruptable = `#!/bin/sh

sig() {
    :
}

trap sig 2 15

/bin/sleep 5
/bin/sleep 5
`

func TestRunCmd(t *testing.T) {
	t.Parallel()
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

func TestRunFail(t *testing.T) {
	t.Parallel()
	cmd := &exec.Cmd{}
	err := runCmd(cmd)
	if err == nil {
		t.Errorf("Failed to error in a command.")
	}
}

func TestNoInterrupt(t *testing.T) {
	t.Parallel()
	os.Mkdir("scripts", 0777)
	err := ioutil.WriteFile("scripts/uninterruptable", []byte(uninterruptable), 0755)
	if err != nil {
		t.Fatalf("Can't create test script.")
	}

	*timeout = time.Second
	*graceTimeout = time.Second
	cmd := exec.Command("./scripts/uninterruptable")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runCmd(cmd); err == nil {
		t.Errorf("Expected error from uninterruptable")
	} else {
		t.Logf("Error was %v", err)
	}
}

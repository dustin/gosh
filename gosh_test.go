package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"sort"
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

func exists(o string) bool {
	_, err := os.Stat(o)
	return err == nil
}

func findCmd(options ...string) string {
	for _, o := range options {
		if exists(o) {
			return o
		}
	}
	return "/not/found"
}

func TestRunCmd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cmd         string
		args        []string
		shouldError bool
	}{
		{findCmd("/usr/bin/false", "/bin/false"), nil, true},
		{findCmd("/usr/bin/true", "/bin/true"), nil, false},
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
	os.Mkdir("tmp", 0777)
	err := ioutil.WriteFile("tmp/uninterruptable", []byte(uninterruptable), 0755)
	if err != nil {
		t.Fatalf("Can't create test script.")
	}
	defer os.Remove("tmp/uninterruptable")

	*timeout = time.Second
	*graceTimeout = time.Second
	cmd := exec.Command("./tmp/uninterruptable")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := runCmd(cmd); err == nil {
		t.Errorf("Expected error from uninterruptable")
	} else {
		t.Logf("Error was %v", err)
	}
}

func TestFindScripts(t *testing.T) {
	t.Parallel()

	scripts, err := findScripts("tmp/test-find-non-existent")
	if err == nil {
		t.Errorf("Failed to fail to find missing scripts, got %v", scripts)
	}

	scripts, err = findScripts("gosh.go")
	if err == nil {
		t.Errorf("Failed to fail to find missing scripts, got %v", scripts)
	}

	os.MkdirAll("tmp/test-find", 0777)
	names := []string{"script1", "script2", "script3"}
	defer os.RemoveAll("tmp/test-find")
	for _, fn := range names {
		err := ioutil.WriteFile("tmp/test-find/"+fn, nil, 0755)
		if err != nil {
			t.Fatalf("Can't create test script.")
		}
	}

	scripts, err = findScripts("tmp/test-find")
	if err != nil {
		t.Fatalf("Failed to find missing scripts, got %v", err)
	}

	sort.Strings(scripts)
	if !reflect.DeepEqual(scripts, names) {
		t.Errorf("Got %v, wanted %v", scripts, names)
	}
}

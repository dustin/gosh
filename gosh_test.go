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

var (
	falseCmd = findCmd("/usr/bin/false", "/bin/false")
	trueCmd  = findCmd("/usr/bin/true", "/bin/true")
)

func TestRunCmd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cmd         string
		args        []string
		shouldError bool
	}{
		{falseCmd, nil, true},
		{trueCmd, nil, false},
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

func TestRun(t *testing.T) {
	t.Parallel()

	if err := run(trueCmd); err != nil {
		t.Errorf("True failed: %v", err)
	}
	if err := run(falseCmd); err == nil {
		t.Errorf("False unexpected succeeded")
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

func TestMkScriptChans(t *testing.T) {
	t.Parallel()

	m1, m2, err := mkScriptChans("tmp/test-find-non-existent")
	if err == nil {
		t.Errorf("Failed to fail to find missing scripts, got %v / %v", m1, m2)
	}

	os.MkdirAll("tmp/test-mkscr", 0777)
	names := []string{"script1", "script2", "script3"}
	defer os.RemoveAll("tmp/test-mkscr")
	for _, fn := range names {
		err := ioutil.WriteFile("tmp/test-mkscr/"+fn, nil, 0755)
		if err != nil {
			t.Fatalf("Can't create test script.")
		}
	}

	chans, chanmap, err := mkScriptChans("tmp/test-mkscr")
	if err != nil {
		t.Fatalf("Failed to find missing scripts, got %v", err)
	}
	if len(chans) != len(chanmap) {
		t.Errorf("Huh? %v %v", chans, chanmap)
	}
	expmap := map[string]string{
		"script1": "tmp/test-mkscr/script1",
		"script2": "tmp/test-mkscr/script2",
		"script3": "tmp/test-mkscr/script3",
	}
	if len(chanmap) != len(expmap) {
		t.Errorf("chanmap != expmap: %v, %v", chanmap, expmap)
	}

	for k, v := range expmap {
		got := chanmap[k]
		if got != v {
			t.Errorf("Error on %q, wanted %q, got %q", k, v, got)
		}
		if _, ok := chans[k]; !ok {
			t.Errorf("No corresponding channel for %q", k)
		}
	}
}

func TestRunner(t *testing.T) {
	t.Parallel()

	chan1 := make(chan string)
	chan2 := make(chan string)
	chan3 := make(chan string)
	chans := map[string]chan string{"test1": chan1, "test2": chan2, "test3": chan3}

	outch := make(chan string, 10)
	h := &httpHandler{chans, nil}
	go h.run(func(s string) error {
		outch <- s
		return nil
	})

	chan1 <- "chan1"
	chan1 <- "chan1"
	chan1 <- "chan1"
	chan3 <- "chan3"
	close(chan1)

	chan1n := 0
	chan3n := 0
	for i := 0; i < 4; i++ {
		s := <-outch
		switch s {
		case "chan1":
			chan1n++
		case "chan3":
			chan3n++
		default:
			t.Errorf("Unexpected message: %v", s)
		}
	}

	if chan1n != 3 {
		t.Errorf("Expected three messages on chan1, got %v", chan1n)
	}
	if chan3n != 1 {
		t.Errorf("Expected one messages on chan3, got %v", chan3n)
	}
}

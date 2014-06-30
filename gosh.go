package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"
)

var (
	timeout = flag.Duration("timeout",
		time.Minute*15, "Maximum time a script can run")
	graceTimeout = flag.Duration("gracetimeout",
		time.Second*5, "Grace period waiting for interrupted cmd to exit")
	bindAddr   = flag.String("addr", ":8888", "http listen address")
	prefixPath = flag.String("path", "/", "path to trigger scripts")

	timeoutError = errors.New("timed out")
)

func waitTimeout(cmd *exec.Cmd, to time.Duration) error {
	// A buffered channel is used here because we *might* not
	// actually read from the channel in the select, in which case
	// the anonymous goroutine would be stuck trying to send
	// forever.
	ch := make(chan error, 1)
	go func() { ch <- cmd.Wait() }()

	select {
	case e := <-ch:
		return e
	case <-time.After(to):
		return timeoutError
	}
}

func runCmd(cmd *exec.Cmd) error {
	err := cmd.Start()
	if err != nil {
		return err
	}

	err = waitTimeout(cmd, *timeout)
	if err == timeoutError {
		cmd.Process.Signal(os.Interrupt)
		if waitTimeout(cmd, *graceTimeout) == timeoutError {
			log.Printf("Timed out waiting for grace period")
			cmd.Process.Kill()
		}
	}
	return err
}

func runner(chs map[string]chan string) {
	cases := []reflect.SelectCase{}
	for _, ch := range chs {
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch)})
	}

	for {
		// Grab the next request (arbitrarily if there's more
		// than one waiting)
		_, val, _ := reflect.Select(cases)
		cmdPath := val.String()

		log.Printf("Got request, executing %v", cmdPath)
		cmd := exec.Command(cmdPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := runCmd(cmd); err != nil {
			log.Printf("Run error: %v", err)
		}
	}
}

func findScripts(dn string) []string {
	d, err := os.Open(dn)
	if err != nil {
		log.Fatalf("Can't open script dir: %q", err)
	}
	defer d.Close()
	names, err := d.Readdirnames(1024)
	if err != nil {
		log.Fatalf("Can't read directory names: %v", err)
	}
	return names
}

func main() {
	flag.Parse()

	chs := map[string]chan string{}
	cmdMap := map[string]string{} // URL path -> filesystem path

	for _, n := range findScripts(flag.Arg(0)) {
		chs[n] = make(chan string, 1)
		cmdMap[n] = filepath.Join(flag.Arg(0), n)
	}

	go runner(chs)

	http.HandleFunc(*prefixPath, func(w http.ResponseWriter, r *http.Request) {
		urlPath := r.URL.Path[1:]
		select {
		case chs[urlPath] <- cmdMap[urlPath]:
			// successfully queued a request to run a script
		default:
			// One of two things happened:
			// 1. The path doesn't actually match to a script,
			//    so there's nothing to queue.
			// 2. The buffer is full, meaning there's already a
			//    run queued that will begin after this request,
			//    so we don't need to do anything.
		}
		w.WriteHeader(202)
	})
	log.Fatal(http.ListenAndServe(*bindAddr, nil))
}

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

	errTimeout = errors.New("timed out")
)

func waitTimeout(ch chan error, to time.Duration) error {
	select {
	case e := <-ch:
		return e
	case <-time.After(to):
		return errTimeout
	}
}

func runCmd(cmd *exec.Cmd) error {
	err := cmd.Start()
	if err != nil {
		return err
	}

	// A buffered channel is used here because we *might* not
	// actually read from the channel in the select, in which case
	// the anonymous goroutine would be stuck trying to send
	// forever.
	ch := make(chan error, 1)
	go func() { ch <- cmd.Wait() }()

	err = waitTimeout(ch, *timeout)
	if err == errTimeout {
		cmd.Process.Signal(os.Interrupt)
		if waitTimeout(ch, *graceTimeout) == errTimeout {
			log.Printf("Timed out waiting for grace period")
			cmd.Process.Kill()
		}
	}
	return err
}

func run(cmdPath string) error {
	cmd := exec.Command(cmdPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := runCmd(cmd)
	if err != nil {
		log.Printf("Run error: %v", err)
	}
	return err
}

func findScripts(dn string) ([]string, error) {
	d, err := os.Open(dn)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	names, err := d.Readdirnames(1024)
	if err != nil {
		return nil, err
	}
	return names, nil
}

func mkScriptChans(dir string) (map[string]chan string, map[string]string, error) {
	chs := map[string]chan string{}
	cmdMap := map[string]string{} // URL path -> filesystem path

	scripts, err := findScripts(dir)
	if err != nil {
		return nil, nil, err
	}

	for _, n := range scripts {
		chs[n] = make(chan string, 1)
		cmdMap[n] = filepath.Join(dir, n)
	}
	return chs, cmdMap, nil
}

type httpHandler struct {
	chs    map[string]chan string
	cmdMap map[string]string
}

func (h httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path[1:]
	select {
	case h.chs[urlPath] <- h.cmdMap[urlPath]:
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
}

func (h httpHandler) run(runFunc func(string) error) {
	cases := []reflect.SelectCase{}
	for _, ch := range h.chs {
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch)})
	}

	for {
		// Grab the next request (arbitrarily if there's more
		// than one waiting)
		_, val, ok := reflect.Select(cases)
		if !ok {
			return
		}
		cmdPath := val.String()

		log.Printf("Got request, executing %v", cmdPath)
		runFunc(cmdPath)
	}
}

func main() {
	flag.Parse()

	chs, cmdMap, err := mkScriptChans(flag.Arg(0))
	if err != nil {
		log.Fatalf("Error finding scripts: %v", err)
	}

	h := &httpHandler{chs, cmdMap}
	go h.run(run)

	http.Handle(*prefixPath, h)
	log.Fatal(http.ListenAndServe(*bindAddr, h))
}

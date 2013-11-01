package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var (
	timeout = flag.Duration("timeout",
		time.Minute*15, "Maximum time a script can run")
	graceTimeout = flag.Duration("timeout",
		time.Second*5, "Grace period waiting for interrupted cmd to exit")
	timeoutError = errors.New("timed out")
)

func waitTimeout(cmd *exec.Cmd, to time.Duration) error {
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

func runner(ch chan string) {
	for cmd := range ch {
		log.Printf("Got request, executing %v", cmd)
		cmd := exec.Command(cmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := runCmd(cmd); err != nil {
			log.Printf("Run error: %v", err)
		}
	}
}

func triggerer(n string, chin <-chan bool, chout chan<- string) {
	for _ = range chin {
		chout <- n
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
	addr := flag.String("addr", ":8888", "http listen address")
	path := flag.String("path", "/", "path to trigger scripts")
	flag.Parse()

	ch := make(chan string)
	chs := map[string]chan bool{}

	for _, n := range findScripts(flag.Arg(0)) {
		chs[n] = make(chan bool)
		go triggerer(filepath.Join(flag.Arg(0), n), chs[n], ch)
	}

	go runner(ch)

	http.HandleFunc(*path, func(w http.ResponseWriter, r *http.Request) {
		select {
		case chs[r.URL.Path[1:]] <- true:
		default:
		}
		w.WriteHeader(202)
	})
	log.Fatal(http.ListenAndServe(*addr, nil))
}

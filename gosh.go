package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func runner(ch chan string) {
	for cmd := range ch {
		log.Printf("Got request, executing %v", cmd)
		cmd := exec.Command(cmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
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

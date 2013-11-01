package main

import (
	"flag"
	"log"
	"net/http"
	"os/exec"
)

func runner(ch chan bool, cmd string, args ...string) {
	for _ = range ch {
		log.Printf("Got request, executing %v", cmd)
		cmd := exec.Command(cmd, args...)
		err := cmd.Run()
		if err != nil {
			log.Printf("Run error: %v", err)
		}
	}
}

func main() {
	addr := flag.String("addr", ":8888", "http listen address")
	path := flag.String("path", "/", "path to trigger script")
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatalf("Need to know what to run")
	}

	ch := make(chan bool, 1)
	go runner(ch, flag.Arg(0), flag.Args()[1:]...)
	http.HandleFunc(*path, func(w http.ResponseWriter, r *http.Request) {
		select {
		case ch <- true:
		default:
			// If we didn't write, it's because the
			// buffer's full, which guarantees another
			// build will occur after this request anyway.
		}
		w.WriteHeader(202)
	})
	log.Fatal(http.ListenAndServe(*addr, nil))
}

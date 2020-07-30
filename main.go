package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/bruno-anjos/solution-utils/http_utils"
	log "github.com/sirupsen/logrus"
)

const (
	serviceName = "ARCHIMEDES"
	host        = "localhost"

	Port = 50000
)

func main() {
	rand.Seed(time.Now().UnixNano())

	debug := flag.Bool("d", false, "add debug logs")
	flag.Parse()

	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	log.Debug("starting log in debug mode")

	addr := fmt.Sprintf("%s:%d", host, Port)

	r := http_utils.NewRouter(PrefixPath, routes)

	log.Infof("Starting %s server in port %d...\n", serviceName, Port)
	log.Fatal(http.ListenAndServe(addr, r))
}

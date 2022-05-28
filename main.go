package main

import (
	"log"
	"net/http"
	"os"
	"path"

	"github.com/boltdb/bolt"
)

func main() {
	cfg := getConfig()

	dataDir := "data"
	err := os.MkdirAll(dataDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Could not create data directory: %s", err)
	}

	db, err := bolt.Open(path.Join(dataDir, "/data"+cfg.id), 0600, nil)
	if err != nil {
		log.Fatalf("Could not open bolt db: %s", err)
	}
	defer db.Close()

	// Clean up old data
	pe := newPgEngine(db)
	// Start off in clean state
	pe.delete()

	// Init raft
	pf := &pgFsm{pe}
	r, err := setupRaft(path.Join(dataDir, "raft"+cfg.id), cfg.id, "localhost:"+cfg.raftPort, pf)
	if err != nil {
		log.Fatal(err)
	}

	// Init http server
	hs := httpServer{r}
	http.HandleFunc("/add-follower", hs.addFollowerHandler)
	go func() {
		err := http.ListenAndServe(":"+cfg.httpPort, nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Init pg server
	runPgServer(cfg.pgPort, db, r)
}

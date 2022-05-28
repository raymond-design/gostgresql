package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/boltdb/bolt"
	"github.com/hashicorp/raft"
	"github.com/jackc/pgproto3/v2"
	pgquery "github.com/pganalyze/pg_query_go/v2"
)

type pgConn struct {
	conn net.Conn
	db   *bolt.DB
	r    *raft.Raft
}

func runPgServer(port string, db *bolt.DB, r *raft.Raft) {
	ln, err := net.Listen("tcp", "localhost:"+port)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}

		pc := pgConn{conn, db, r}
		go pc.handle()
	}
}

func (pc pgConn) handle() {
	pgc := pgproto3.NewBackend(pgproto3.NewChunkReader(pc.conn), pc.conn)
	defer pc.conn.Close()

	err := pc.handleStartupMessage(pgc)
	if err != nil {
		log.Println(err)
		return
	}

	for {
		err := pc.handleMessage(pgc)
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func (pc pgConn) handleStartupMessage(pgconn *pgproto3.Backend) error {
	startupMessage, err := pgconn.ReceiveStartupMessage()
	if err != nil {
		return fmt.Errorf("Error receiving startup message: %s", err)
	}

	switch startupMessage.(type) {
	case *pgproto3.StartupMessage:
		buf := (&pgproto3.AuthenticationOk{}).Encode(nil)
		buf = (&pgproto3.ReadyForQuery{TxStatus: 'I'}).Encode(buf)
		_, err = pc.conn.Write(buf)
		if err != nil {
			return fmt.Errorf("Error sending ready for query: %s", err)
		}

		return nil
	case *pgproto3.SSLRequest:
		_, err = pc.conn.Write([]byte("N"))
		if err != nil {
			return fmt.Errorf("Error sending deny SSL request: %s", err)
		}

		return pc.handleStartupMessage(pgconn)
	default:
		return fmt.Errorf("Unknown startup message: %#v", startupMessage)
	}
}

func (pc pgConn) handleMessage(pgc *pgproto3.Backend) error {
	msg, err := pgc.Receive()
	if err != nil {
		return fmt.Errorf("Error receiving message: %s", err)
	}

	switch t := msg.(type) {
	case *pgproto3.Query:
		stmts, err := pgquery.Parse(t.String)
		if err != nil {
			return fmt.Errorf("Error parsing query: %s", err)
		}

		if len(stmts.GetStmts()) > 1 {
			return fmt.Errorf("Only make one request at a time.")
		}

		stmt := stmts.GetStmts()[0]

		// Handle SELECTs here
		s := stmt.GetStmt().GetSelectStmt()
		if s != nil {
			pe := newPgEngine(pc.db)
			res, err := pe.executeSelect(s)
			if err != nil {
				return err
			}

			pc.writePgResult(res)
			return nil
		}
		// Otherwise it's DDL/DML, raftify
		future := pc.r.Apply([]byte(t.String), 500*time.Millisecond)
		if err := future.Error(); err != nil {
			return fmt.Errorf("Could not apply: %s", err)
		}

		e := future.Response()
		if e != nil {
			return fmt.Errorf("Could not apply (internal): %s", e)
		}

		pc.done(nil, strings.ToUpper(strings.Split(t.String, " ")[0])+" ok")
	case *pgproto3.Terminate:
		return nil
	default:
		return fmt.Errorf("Received message other than Query from client: %s", msg)
	}

	return nil
}

func (pc pgConn) done(buf []byte, msg string) {
	buf = (&pgproto3.CommandComplete{CommandTag: []byte(msg)}).Encode(buf)
	buf = (&pgproto3.ReadyForQuery{TxStatus: 'I'}).Encode(buf)
	_, err := pc.conn.Write(buf)
	if err != nil {
		log.Printf("Failed to write query response: %s", err)
	}
}

var dataTypeOIDMap = map[string]uint32{
	"text":            25,
	"pg_catalog.int4": 23,
}

func (pc pgConn) writePgResult(res *pgResult) {
	rd := &pgproto3.RowDescription{}
	for i, field := range res.fieldNames {
		rd.Fields = append(rd.Fields, pgproto3.FieldDescription{
			Name:        []byte(field),
			DataTypeOID: dataTypeOIDMap[res.fieldTypes[i]],
		})
	}
	buf := rd.Encode(nil)
	for _, row := range res.rows {
		dr := &pgproto3.DataRow{}
		for _, value := range row {
			bs, err := json.Marshal(value)
			if err != nil {
				log.Printf("Failed to marshal cell: %s\n", err)
				return
			}

			dr.Values = append(dr.Values, bs)
		}

		buf = dr.Encode(buf)
	}

	pc.done(buf, fmt.Sprintf("SELECT %d", len(res.rows)))
}

package main

import (
	"fmt"
	"io"

	"github.com/hashicorp/raft"
)

type snapshotNoop struct{}

func (sn snapshotNoop) Persist(sink raft.SnapshotSink) error {
	return sink.Cancel()
}

func (sn snapshotNoop) Release() {}

func (pf *pgFsm) Snapshot() (raft.FSMSnapshot, error) {
	return snapshotNoop{}, nil
}

func (pf *pgFsm) Restore(rc io.ReadCloser) error {
	return fmt.Errorf("Nothing to restore")
}

// Sql table definition
type tableDefinition struct {
	Name        string
	ColumnNames []string
	ColumnTypes []string
}

type pgResult struct {
	fieldNames []string
	fieldTypes []string
	rows       [][]interface{}
}

package main

import (
	"github.com/hashicorp/raft"
)

type pgFsm struct {
	pe *pgEngine
}

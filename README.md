gostgresql
-------
gostgresql is a working SQL database that uses bbolt, Hashicorp raft, and the postgres parser. It's basically a budget version of CockroachDB, but ~700 loc.

Usage
-------
In one terminal:
`make t1` 
In a second terminal:
`make t2` 
In a third terminal:
`make t3` 
and then enter the db in the same terminal:
`psql -h localhost -p 6000`
now try some sql commands
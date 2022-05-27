t1:
	go build
	./gostgresql --node-id node1 --raft-port 2222 --http-port 8222 --pg-port 6000

t2:
	go build
	./gostgresql --node-id node2 --raft-port 2223 --http-port 8223 --pg-port 6001

t3:
	curl 'localhost:8222/add-follower?addr=localhost:2223&id=node2'
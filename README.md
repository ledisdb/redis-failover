# redis-failover

Automatic redis monitoring and failover. 

## Why not redis-sentinel?

redis-sentinel is very powerful, why not use it but build a new one?

1. I want to support not only redis, but also [LedisDB](https://github.com/siddontang/ledisdb)'s monitoring and failover.
2. I want to embed it into another service like [xcodis](https://github.om/siddontang/xcodis).
3. I just want to learn how to build a distributed service using [raft](https://raftconsensus.github.io). :-)

## Install and usage

```
go get github.com/siddontang/redis-failover
```

First you must start redis and build the replication topology by yourself. e.g, 127.0.0.1:6379 is master and 127.0.0.1:6380 is slave.

### Use without raft, only single node

```
redis-failover -addr=127.0.0.1:11000 -masters=127.0.0.1:6379
```

`addr` is redis-failover HTTP listen address, `masters` is the master redis server you want to monitor. 

you can use a config file too, like `redis-failover -config=./etc/failover.toml`. see [failover.toml](./etc/failover.toml).

You can add master dynamically from HTTP, using [httpie](https://github.com/jakubroztocil/httpie) below:

```
http POST :11000/master masters==127.0.0.1:6379
```

### Use raft, but only single node

```
redis-failover -addr=127.0.0.1:11000 -masters=127.0.0.1:6379 -raft_addr=127.0.0.1:12000 -raft_data_dir=./var0 -raft_cluster=127.0.0.1:12000
```

`raft_addr` is the raft listen address for inner raft communication. `raft_data_dir` is the store path for raft, `raft_cluster` is the raft cluster, here only one node. 

You must know that if you want to use raft to avoid redis-failover single point of failure, you should not use only one raft node in production.

### Use raft, with multi nodes

```
redis-failover -addr=127.0.0.1:11000 -masters=127.0.0.1:6379 -raft_addr=127.0.0.1:12000 -raft_data_dir=./var0 -raft_cluster=127.0.0.1:12000,127.0.0.1:12001,127.0.0.1:12002

redis-failover -addr=127.0.0.1:11001 -masters=127.0.0.1:6379 -raft_addr=127.0.0.1:12001 -raft_data_dir=./var1 -raft_cluster=127.0.0.1:12000,127.0.0.1:12001,127.0.0.1:12002

redis-failover -addr=127.0.0.1:11002 -masters=127.0.0.1:6379 -raft_addr=127.0.0.1:12002 -raft_data_dir=./var2 -raft_cluster=127.0.0.1:12000,127.0.0.1:12001,127.0.0.1:12002
```

`raft_cluster` now contains three raft nodes, so if one node down, other two can still work correctly. 

## Failover

After you start redis-failover and set master redis, redis-failover will check it automatically. After it finds the master is down, it will do failover, the failover step is:

1. Elect a slave which has the most up-to-date data with master to the candidate, use `INFO REPLICATION` to check.
2. Promote the candidate to the master, use `SLAVEOF NO ONE`.
3. Let other slaves replicate from the new master, use `SLAVEOF new_master_host new_master_port`.

redis-failover will log some messages for failover, like:

```
[2015/02/10 14:10:18] group.go:64 [Error] do ROLE command for 127.0.0.1:6379 error: EOF, try again
[2015/02/10 14:10:18] group.go:56 [Error] dial 127.0.0.1:6379 error: dial tcp 127.0.0.1:6379: connection refused, try again
[2015/02/10 14:10:18] group.go:56 [Error] dial 127.0.0.1:6379 error: dial tcp 127.0.0.1:6379: connection refused, try again
[2015/02/10 14:10:18] app.go:166 [Error] check master 127.0.0.1:6379 err Node is down, do failover
[2015/02/10 14:10:18] group.go:259 [Info] select slave 127.0.0.1:6380 as new master, priority:100, repl_offset:29
```

If the failover failed, redis-failover will stop to check this redis to avoid future unexpected errors, so at that time, you may fix it manually by yourself. 

## Todo

+ Support zookeeper or etcd, now redis-failover uses raft to select the leader and do monitoring and failover, but it's easy to support zookeeper or etcd.
+ Integrate redis-failover into [xcodis](https://github.om/siddontang/xcodis) or even [codis](http://github.com/wandoulabs/codis).

## Feedback

Email: siddontang@gmail.com 


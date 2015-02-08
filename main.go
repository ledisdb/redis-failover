package main

import (
	"flag"
	"fmt"
	"github.com/siddontang/redis-failover/failover"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var configFile = flag.String("config", "", "failover config file")
var addr = flag.String("addr", ":11000", "failover http listen addr")
var dataDir = flag.String("data_dir", "./var", "data store path")
var logDir = flag.String("log_dir", "./log", "log store path")
var raftAddr = flag.String("raft_addr", "", "raft listen addr, if empty, we will disable raft")
var cluster = flag.String("cluster", "", "raft cluster,vseperated by comma")
var clusterState = flag.String("cluster_state", "", "new or existing, if new, we will deprecate old saved cluster and use new")
var masters = flag.String("masters", "", "redis master need to be monitored, seperated by comma")
var mastersState = flag.String("masters_state", "", "new or existing for raft, if new, we will depracted old saved masters")

func main() {
	flag.Parse()

	var c *failover.Config
	var err error
	if len(*configFile) > 0 {
		c, err = failover.NewConfigWithFile(*configFile)
		if err != nil {
			fmt.Printf("load failover config %s err %v", *configFile, err)
			return
		}
	} else {
		fmt.Printf("no config file, use default config")
		c = new(failover.Config)
		c.Addr = ":11000"
		c.DataDir = "./var"
	}

	if len(*raftAddr) > 0 {
		c.RaftAddr = *raftAddr
	}

	if len(*dataDir) > 0 {
		c.DataDir = *dataDir
	}

	if len(*logDir) > 0 {
		c.LogDir = *logDir
	}

	seps := strings.Split(*cluster, ",")
	if len(seps) > 0 {
		c.Cluster = seps
	}

	if len(*clusterState) > 0 {
		c.ClusterState = *clusterState
	}

	seps = strings.Split(*masters, ",")
	if len(seps) > 0 {
		c.Masters = seps
	}

	if len(*mastersState) > 0 {
		c.MastersState = *mastersState
	}

	app, err := failover.NewApp(c)
	if err != nil {
		fmt.Printf("new failover app error %v", err)
		return
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		os.Kill,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		<-sc
		app.Close()
	}()

	app.Run()
}

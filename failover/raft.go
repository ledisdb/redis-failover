package failover

import (
	"fmt"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-boltdb"
	"github.com/siddontang/go/log"
	"io"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

type FSM struct {
}

func (fsm *FSM) Apply(log *raft.Log) interface{} {
	return nil
}

func (fsm *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil
}

func (fsm *FSM) Restore(inp io.ReadCloser) error {
	return nil
}

type Snapshot struct {
}

func (snap *Snapshot) Persist(sink raft.SnapshotSink) error {
	return nil
}

func (snap *Snapshot) Release() {

}

// redis-failover uses raft to elect the cluster leader and do monitoring and failover.
type Raft struct {
	r *raft.Raft

	log     *os.File
	dbStore *raftboltdb.BoltStore
	trans   *raft.NetworkTransport
}

func newRaft(c *Config, fsm raft.FSM) (*Raft, error) {
	if len(c.Cluster) == 0 {
		log.Info("no cluster in config, don't use raft")
		return nil, nil
	}

	var raftListen string
	peers := make([]net.Addr, 0, len(c.Cluster))
	for _, cluster := range c.Cluster {
		seps := strings.SplitN(cluster, ":", 1)
		if len(seps) != 2 {
			return nil, fmt.Errorf("invalid cluster format %s, must ID:host:port", cluster)
		}

		id, err := strconv.Atoi(seps[0])
		if err != nil {
			return nil, fmt.Errorf("invalid cluster format %s, must ID:host:port", cluster)
		} else if id == c.ServerID {
			if len(raftListen) > 0 {
				return nil, fmt.Errorf("duplicate cluster config for ID %d", id)
			}
			raftListen = seps[1]
		}

		a, err := net.ResolveTCPAddr("tcp", seps[1])
		if err != nil {
			return nil, fmt.Errorf("invalid cluster format %s, must ID:host:port, err:%v", cluster, err)
		}

		peers = append(peers, a)
	}

	if len(raftListen) == 0 {
		return nil, fmt.Errorf("must have a cluster config for current server id %d", c.ServerID)
	}

	r := new(Raft)
	raftPath := path.Join(c.DataDir, "raft")

	os.MkdirAll(raftPath, 0755)

	var err error

	cfg := raft.DefaultConfig()

	if len(c.LogDir) == 0 {
		r.log = os.Stdout
	} else {
		logFile := path.Join(raftPath, "raft.log")
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
		if err != nil {
			return nil, err
		}
		r.log = f

		cfg.LogOutput = r.log
	}

	raftDBPath := path.Join(raftPath, "db")
	r.dbStore, err = raftboltdb.NewBoltStore(raftDBPath)
	if err != nil {
		return nil, err
	}

	fileStore, err := raft.NewFileSnapshotStore(raftPath, 1, r.log)
	if err != nil {
		return nil, err
	}

	r.trans, err = raft.NewTCPTransport(raftListen, nil, 3, 5*time.Second, r.log)
	if err != nil {
		return nil, err
	}

	peerStore := raft.NewJSONPeers(raftPath, r.trans)

	if c.ClusterState == ClusterStateNew {
		log.Infof("cluster state is new, use new cluster config")
		peerStore.SetPeers(peers)
	} else {
		log.Infof("cluster state is existing, use previous + new cluster config")
		ps, err := peerStore.Peers()
		if err != nil {
			log.Errorf("get store peers error %v", err)
			return nil, err
		}

		for _, peer := range peers {
			peerStore.SetPeers(raft.AddUniquePeer(ps, peer))
		}
	}

	if peers, _ := peerStore.Peers(); len(peers) <= 1 {
		cfg.EnableSingleNode = true
		log.Warn("raft will run in single node mode")
	}

	r.r, err = raft.NewRaft(cfg, fsm, r.dbStore, r.dbStore, fileStore, peerStore, r.trans)

	return r, err
}

func (r *Raft) Close() {
	if r.trans != nil {
		r.trans.Close()
	}

	if r.r != nil {
		future := r.r.Shutdown()
		if err := future.Error(); err != nil {
			log.Errorf("Error shutting down raft: %v", err)
		}
	}

	if r.dbStore != nil {
		r.dbStore.Close()
	}

	if r.log != os.Stdout {
		r.log.Close()
	}
}

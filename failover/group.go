package failover

import (
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/siddontang/go/log"
)

const (
	MasterType = "master"
	SlaveType  = "slave"
)

const (
	ConnectState    = "connect"
	ConnectingState = "connecting"
	ConnectedState  = "connected"
	SyncState       = "sync"
)

// A node represents a real redis server
type Node struct {
	// master or slave
	Type string

	// Redis address, only support tcp now
	Addr string

	// Replication offset
	ReplOffset int

	// Replication state for slave
	// connect, connecting, connected, sync
	State string

	conn redis.Conn
}

func (n *Node) doRole() ([]interface{}, error) {
	var err error
	var v []interface{}
	for i := 0; i < 3; i++ {
		if n.conn == nil {
			// todo, dail timeout
			n.conn, err = redis.Dial("tcp", n.Addr)
			if err != nil {
				log.Errorf("dial %s error: %v, try again", n.Addr, err)
				continue
			}

		}

		v, err = redis.Values(n.conn.Do("ROLE"))
		if err != nil {
			log.Errorf("do role command for %s error: %v, try again", n.Addr, err)
			n.conn.Close()
			n.conn = nil
			continue
		} else {
			return v, nil
		}
	}

	// go here means do role command error, maybe redis is down.
	return nil, err
}

func (n *Node) Close() {
	if n.conn != nil {
		n.conn.Close()
		n.conn = nil
	}
}

// A group contains a Redis master and one or more slaves
// It will use role command per second to check master's alive
// and find slaves automatically.
type Group struct {
	Master Node
	Slaves []Node
}

func NewGroup(masterAddr string) (*Group, error) {
	g := new(Group)

	return g, nil
}

func (g *Group) check() error {

}

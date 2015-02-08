package failover

import (
	"errors"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"github.com/siddontang/go/log"
	"net"
	"sync"
	"time"
)

var (
	ErrNodeDown    = errors.New("Node is down")
	ErrNodeAlive   = errors.New("Node may be still alive")
	ErrNoCandidate = errors.New("no proper candidate to be promoted to master")
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

type FailoverHandler func(oldMaster, newMaster string) error

// A node represents a real redis server
type Node struct {
	// Redis address, only support tcp now
	Addr string

	conn redis.Conn
}

func (n Node) String() string {
	return n.Addr
}

func (n *Node) doCommand(cmd string, args ...interface{}) (interface{}, error) {
	var err error
	var v interface{}
	for i := 0; i < 3; i++ {
		if n.conn == nil {
			n.conn, err = redis.DialTimeout("tcp", n.Addr, 5*time.Second, 0, 0)
			if err != nil {
				log.Errorf("dial %s error: %v, try again", n.Addr, err)
				continue
			}

		}

		v, err = n.conn.Do(cmd, args...)
		if err != nil {
			log.Errorf("do %s command for %s error: %v, try again", cmd, n.Addr, err)
			n.conn.Close()
			n.conn = nil
			continue
		} else {
			return v, nil
		}
	}

	// go here means do command error, maybe redis is down.
	return nil, err
}

func (n *Node) doRole() ([]interface{}, error) {
	return redis.Values(n.doCommand("ROLE"))
}

func (n *Node) ping() error {
	_, err := n.doCommand("PING")
	return err
}

func (n *Node) slaveof(host string, port string) error {
	_, err := n.doCommand("SLAVEOF", host, port)
	return err
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
	Master Node            `json:"master"`
	Slaves map[string]Node `json:"slaves"`

	m sync.Mutex

	hMutex   sync.Mutex
	handlers []FailoverHandler
}

func NewGroup(masterAddr string) *Group {
	g := new(Group)

	g.Master = Node{Addr: masterAddr}
	g.Slaves = make(map[string]Node)

	return g
}

func (g *Group) Close() {
	g.m.Lock()
	defer g.m.Unlock()

	g.Master.Close()

	for _, slave := range g.Slaves {
		slave.Close()
	}
}

func (g *Group) Check() error {
	g.m.Lock()
	defer g.m.Unlock()

	v, err := g.Master.doRole()
	if err != nil {
		return ErrNodeDown
	}

	// the first line is server type
	serverType, _ := redis.String(v[0], nil)
	if serverType != MasterType {
		log.Errorf("server %s is not master now", g.Master.Addr)
	}

	// second is master replication offset, skip

	// then slave list [host, port, offset]
	slaves, _ := redis.Values(v[2], nil)
	nodes := make(map[string]Node, len(slaves))
	for i := 0; i < len(slaves); i++ {
		ss, _ := redis.Strings(slaves[i], nil)
		var n Node
		n.Addr = fmt.Sprintf("%s:%s", ss[0], ss[1])
		nodes[n.Addr] = n
	}

	// we don't care slave add or remove too much, so only log
	for addr, _ := range nodes {
		if _, ok := g.Slaves[addr]; !ok {
			log.Infof("slave %s added", addr)
		}
	}

	for addr, slave := range g.Slaves {
		if _, ok := nodes[addr]; !ok {
			log.Infof("slave %s removed", addr)
			slave.Close()
		}
	}

	g.Slaves = nodes
	return nil
}

// failover does the following thing
//  1, check master is still alive or not again
//  2, elect a best slave which has the most up-to-date data with master
//  3, promote the slave to master, then let other slaves replicate from it
func (g *Group) Failover() error {
	g.m.Lock()
	defer g.m.Unlock()

	// first, check master is down again
	if err := g.Master.ping(); err == nil {
		log.Infof("ping master %s OK, may not down, return", g.Master.Addr)
		return nil
	}

	oldMaster := g.Master.Addr

	addr, err := g.elect()
	if err != nil {
		log.Errorf("elect slave error %v", err)
		return err
	}

	if len(addr) == 0 {
		log.Errorf("no proper slave to be promoted")
		return ErrNoCandidate
	}

	log.Infof("elect %s as new master, promote it", addr)

	if err = g.promote(addr); err != nil {
		log.Errorf("promote %s to master error %v", addr, err)
		return err
	}

	g.hMutex.Lock()
	for _, h := range g.handlers {
		if err := h(oldMaster, addr); err != nil {
			log.Errorf("on failover handler error %v", err)
		}
	}
	g.hMutex.Unlock()

	return nil
}

func (g *Group) AddHandler(f FailoverHandler) {
	g.hMutex.Lock()
	g.handlers = append(g.handlers, f)
	g.hMutex.Unlock()
}

func (g *Group) elect() (string, error) {
	var addr string
	var maxOffset int64 = 0

	for _, slave := range g.Slaves {
		v, err := slave.doRole()
		if err != nil {
			//skip this slave
			log.Infof("slave %s do role command err %v, skip it", slave.Addr, err)
			continue
		}

		// the first line is server type
		serverType, _ := redis.String(v[0], nil)
		if serverType != SlaveType {
			log.Errorf("server %s is not slave now", slave.Addr)
		}

		// the second and third is host and port, skip it
		// the fouth is replication state
		state, _ := redis.String(v[3], nil)
		if state == ConnectState || state == SyncState {
			log.Errorf("slave %s replication state is %s, master %s:%v may be not down???",
				slave.Addr, state, v[1], v[2])
			return "", ErrNodeAlive
		}

		// the end is the replication offset
		offset, _ := redis.Int64(v[4], nil)
		if offset > maxOffset {
			addr = slave.Addr
			maxOffset = offset
		}
	}

	return addr, nil
}

func (g *Group) promote(addr string) error {
	node := g.Slaves[addr]

	if err := node.slaveof("no", "one"); err != nil {
		return err
	}

	delete(g.Slaves, addr)

	g.Master = node

	host, port, _ := net.SplitHostPort(addr)
	for _, slave := range g.Slaves {
		if err := slave.slaveof(host, port); err != nil {
			// if we go here, the replication topology may be wrong
			// so use fatal level and we should fix it manually
			log.Fatalf("slaveof %s to master %s err %v", slave.Addr, addr, err)
		} else {
			log.Infof("slaveof %s to master %s ok", slave.Addr, addr)
		}
	}

	return nil
}

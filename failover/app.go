package failover

import (
	"github.com/gorilla/mux"
	"github.com/siddontang/go/log"
	"net"
	"net/http"
	"sync"
	"time"
)

type FailoverHandler func(oldMaster, newMaster string) error

type App struct {
	c *Config

	l net.Listener

	r *Raft

	masters *masterFSM

	groups map[string]*Group

	quit chan struct{}
	wg   sync.WaitGroup

	hMutex   sync.Mutex
	handlers []FailoverHandler
}

func NewApp(c *Config) (*App, error) {
	var err error

	a := new(App)
	a.quit = make(chan struct{})
	a.groups = make(map[string]*Group)

	a.masters = newMasterFSM()

	if len(c.Addr) == 0 {
		a.l, err = net.Listen("tcp", c.Addr)
		if err != nil {
			return nil, err
		}
	}

	a.r, err = newRaft(c, a.masters)
	if err != nil {
		return nil, err
	}

	if c.MastersState == MastersStateNew {
		a.setMasters(c.Masters)
	} else {
		a.addMasters(c.Masters)
	}

	return a, nil
}

func (a *App) Close() {
	if a.l != nil {
		a.l.Close()
	}

	if a.r != nil {
		a.r.Close()
	}

	close(a.quit)

	a.wg.Wait()
}

func (a *App) Run() {
	go a.startHTTP()

	a.wg.Add(1)
	t := time.NewTicker(1 * time.Second)
	defer func() {
		t.Stop()
		a.wg.Done()
	}()

	for {
		select {
		case <-t.C:
			a.check()
		case <-a.quit:
			return
		}
	}
}

func (a *App) check() {
	if a.r != nil && !a.r.IsLeader() {
		// is not leader, not check
		return
	}

	masters := a.masters.GetMasters()

	var wg sync.WaitGroup
	for _, master := range masters {
		g, ok := a.groups[master]
		if !ok {
			g = newGroup(master)
			a.groups[master] = g
		}

		wg.Add(1)
		go a.checkMaster(&wg, g)
	}
}

func (a *App) checkMaster(wg *sync.WaitGroup, g *Group) {
	defer wg.Done()
	err := g.Check()
	if err == nil {
		return
	}

	oldMaster := g.Master.Addr

	// remove it from saved masters
	a.delMasters([]string{oldMaster})

	if err == ErrNodeType {
		log.Errorf("server %s is not master now, we will skip it", oldMaster)
		return
	}

	log.Errorf("check master %s err %v, do failover", oldMaster, err)

	newMaster, err := g.Failover()
	if err != nil {
		log.Errorf("do master %s failover err: %v", oldMaster, err)
		return
	}

	a.hMutex.Lock()
	for _, h := range a.handlers {
		err = h(oldMaster, newMaster)
		log.Errorf("do failover handler err: %v", err)
	}
	a.hMutex.Unlock()
}

func (a *App) startHTTP() {
	m := mux.NewRouter()

	m.Handle("/master", &masterHandler{a})
	m.Handle("/cluster", &clusterHandler{a})

	s := http.Server{
		Handler: m,
	}

	s.Serve(a.l)
}

func (a *App) addMasters(addrs []string) {
	if a.r != nil {
		a.r.AddMasters(addrs, 0)
	} else {
		a.masters.AddMasters(addrs)
	}
}

func (a *App) delMasters(addrs []string) {
	if a.r != nil {
		a.r.DelMasters(addrs, 0)
	} else {
		a.masters.DelMasters(addrs)
	}
}

func (a *App) setMasters(addrs []string) {
	if a.r != nil {
		a.r.SetMasters(addrs, 0)
	} else {
		a.masters.SetMasters(addrs)
	}

}

func (a *App) AddHandler(f FailoverHandler) {
	a.hMutex.Lock()
	a.handlers = append(a.handlers, f)
	a.hMutex.Unlock()
}

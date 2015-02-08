package failover

import (
	"net/http"
	"strings"
)

type masterHandler struct {
	a *App
}

func (h *masterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		masters := h.a.masters.GetMasters()
		w.Write([]byte(strings.Join(masters, ",")))
	case "POST":
		masters := strings.Split(r.FormValue("masters"), ",")
		h.a.addMasters(masters)
	case "PUT":
		masters := strings.Split(r.FormValue("masters"), ",")
		h.a.setMasters(masters)
	case "DELETE":
		masters := strings.Split(r.FormValue("masters"), ",")
		h.a.delMasters(masters)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

type clusterHandler struct {
	a *App
}

func (h *clusterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.a.r == nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("raft cluster is not supported"))
		return
	}

	switch r.Method {
	case "GET":
		peers, err := h.a.r.GetPeers()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		w.Write([]byte(strings.Join(peers, ",")))
	case "POST":
		peers := strings.Split(r.FormValue("peers"), ",")
		for _, peer := range peers {
			h.a.r.AddPeer(peer)
		}
	case "PUT":
		peers := strings.Split(r.FormValue("peers"), ",")
		h.a.r.SetPeers(peers)
	case "DELETE":
		peers := strings.Split(r.FormValue("peers"), ",")
		for _, peer := range peers {
			h.a.r.DelPeer(peer)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/Jaydee94/podustrial/internal/k8s"
)

type Server struct {
	k8sClient *k8s.Client
	hub       *Hub
}

func NewServer(k8sClient *k8s.Client, hub *Hub) *Server {
	if hub == nil {
		panic("api: NewServer called with a nil hub")
	}
	return &Server{k8sClient: k8sClient, hub: hub}
}

func (s *Server) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := s.k8sClient.Healthy(r.Context()); err != nil {
		log.Printf("healthz: cluster unreachable: %v", err)
		http.Error(w, "cluster unreachable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) HandleSpawnAction(w http.ResponseWriter, r *http.Request) {
	var req k8s.SpawnContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		http.Error(w, "id must not be empty", http.StatusBadRequest)
		return
	}
	pod, err := s.k8sClient.SpawnContainer(r.Context(), req)
	if err != nil {
		log.Printf("spawn-container: id=%q: %v", req.ID, err)
		http.Error(w, "failed to spawn container", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"pod": pod.Name})
}

func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.HandleHealthz)
	mux.HandleFunc("POST /api/actions/spawn-container", s.HandleSpawnAction)
	mux.HandleFunc("/ws", s.hub.ServeWS)
	return mux
}

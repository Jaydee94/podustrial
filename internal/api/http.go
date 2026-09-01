package api

import (
	"encoding/json"
	"net/http"

	"github.com/Jaydee94/podustrial/internal/k8s"
)

type Server struct {
	k8sClient *k8s.Client
}

func NewServer(k8sClient *k8s.Client) *Server {
	return &Server{k8sClient: k8sClient}
}

func (s *Server) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := s.k8sClient.Healthy(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	return mux
}

package httpapi

import (
	"net/http"

	"github.com/odurgut/rasat/internal/version"
)

type healthResponse struct {
	OK      bool   `json:"ok"`
	Version string `json:"version"`
}

type readyResponse struct {
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
}

type versionResponse struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := writeJSON(w, http.StatusOK, healthResponse{
		OK:      true,
		Version: version.Version,
	}); err != nil {
		s.log.Error("write health", "err", err, "request_id", requestID(r))
	}
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	err := s.ready.Ready(r.Context())
	if err != nil {
		s.log.Warn("not ready", "err", err, "request_id", requestID(r))
		if werr := writeJSON(w, http.StatusServiceUnavailable, readyResponse{
			OK:     false,
			Reason: "clickhouse",
		}); werr != nil {
			s.log.Error("write ready", "err", werr, "request_id", requestID(r))
		}
		return
	}
	if err := writeJSON(w, http.StatusOK, readyResponse{OK: true}); err != nil {
		s.log.Error("write ready", "err", err, "request_id", requestID(r))
	}
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if err := writeJSON(w, http.StatusOK, versionResponse{
		Version: version.Version,
		Commit:  version.Commit,
	}); err != nil {
		s.log.Error("write version", "err", err, "request_id", requestID(r))
	}
}

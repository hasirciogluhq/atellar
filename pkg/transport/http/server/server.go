// Package server exposes an api.Interface over HTTP/1.1 and cleartext
// HTTP/2 with JSON bodies. Watches stream newline-delimited JSON.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/hasirciogluhq/atellar/pkg/api"
	"github.com/hasirciogluhq/atellar/pkg/api/watch"
	v1 "github.com/hasirciogluhq/atellar/pkg/apis/core/v1"
	"github.com/hasirciogluhq/atellar/pkg/auth/authn"
	"github.com/hasirciogluhq/atellar/pkg/transport/http/wire"
)

// Server is an http.Handler for the Atellar API.
type Server struct {
	api  api.Interface
	auth authn.Authenticator
	mux  *http.ServeMux
}

func New(svc api.Interface, auth authn.Authenticator) *Server {
	s := &Server{api: svc, auth: auth, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) routes() {
	p := wire.Prefix
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok\n") })

	s.mux.HandleFunc("GET "+p+"/nodes", s.listNodes)
	s.mux.HandleFunc("GET "+p+"/nodes/{ref}", s.getNode)
	s.mux.HandleFunc("POST "+p+"/nodes/{ref}/evict", s.evictNode)
	s.mux.HandleFunc("GET "+p+"/nodes/{ref}/overlay-ips", s.listOverlayIPs)
	s.mux.HandleFunc("POST "+p+"/nodes/register", s.registerNode)
	s.mux.HandleFunc("POST "+p+"/nodes/self/heartbeat", s.selfHeartbeat)
	s.mux.HandleFunc("PUT "+p+"/nodes/self/hardware", s.selfHardware)
	s.mux.HandleFunc("POST "+p+"/nodes/self/api-key", s.selfRenewAPIKey)

	s.mux.HandleFunc("GET "+p+"/containers", s.listContainers)
	s.mux.HandleFunc("POST "+p+"/containers", s.createContainer)
	s.mux.HandleFunc("GET "+p+"/containers/{id}", s.getContainer)
	s.mux.HandleFunc("DELETE "+p+"/containers/{id}", s.deleteContainer)
	s.mux.HandleFunc("GET "+p+"/containers/{id}/events", s.listContainerEvents)
	s.mux.HandleFunc("PUT "+p+"/containers/{id}/runtime", s.reportRuntime)
	s.mux.HandleFunc("POST "+p+"/containers/{id}/overlay-ip", s.allocateOverlayIP)

	s.mux.HandleFunc("GET "+p+"/jointokens", s.listJoinTokens)
	s.mux.HandleFunc("POST "+p+"/jointokens", s.createJoinToken)
}

// ServeHTTP authenticates the bearer token and dispatches the request.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tok, err := authn.ParseBearer(r.Header.Get(authn.HeaderAuthorization))
	if err != nil {
		writeError(w, api.Unauthorized("%v", err))
		return
	}
	principal, err := s.auth.Authenticate(r.Context(), tok)
	if err != nil {
		writeError(w, api.Unauthorized("%v", err))
		return
	}
	if principal != nil {
		r = r.WithContext(authn.WithPrincipal(r.Context(), principal))
	}
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe serves until ctx is done, then shuts down gracefully.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ctx, ln)
}

func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Handler:           s,
		Protocols:         protocols,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	err := srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		<-done
		return nil
	}
	return err
}

func (s *Server) listNodes(w http.ResponseWriter, r *http.Request) {
	if isWatch(r) {
		wi, err := s.api.Nodes().Watch(r.Context())
		serveWatch(w, r, wi, err)
		return
	}
	nodes, err := s.api.Nodes().List(r.Context())
	respond(w, http.StatusOK, nodes, err)
}

func (s *Server) getNode(w http.ResponseWriter, r *http.Request) {
	node, err := s.api.Nodes().Get(r.Context(), r.PathValue("ref"))
	respond(w, http.StatusOK, node, err)
}

func (s *Server) evictNode(w http.ResponseWriter, r *http.Request) {
	node, err := s.api.Nodes().Evict(r.Context(), r.PathValue("ref"))
	respond(w, http.StatusOK, node, err)
}

func (s *Server) listOverlayIPs(w http.ResponseWriter, r *http.Request) {
	ips, err := s.api.Nodes().ListOverlayIPs(r.Context(), r.PathValue("ref"), boolQuery(r, "free"))
	respond(w, http.StatusOK, ips, err)
}

func (s *Server) registerNode(w http.ResponseWriter, r *http.Request) {
	var req v1.RegisterNodeRequest
	if !decode(w, r, &req) {
		return
	}
	if req.JoinToken == "" {
		req.JoinToken = r.URL.Query().Get("token")
	}
	resp, err := s.api.Nodes().Register(r.Context(), req)
	respond(w, http.StatusCreated, resp, err)
}

func (s *Server) selfHeartbeat(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusNoContent, nil, s.api.Nodes().SelfHeartbeat(r.Context()))
}

func (s *Server) selfHardware(w http.ResponseWriter, r *http.Request) {
	var hw v1.NodeHardware
	if !decode(w, r, &hw) {
		return
	}
	respond(w, http.StatusNoContent, nil, s.api.Nodes().SelfReportHardware(r.Context(), hw))
}

func (s *Server) selfRenewAPIKey(w http.ResponseWriter, r *http.Request) {
	key, err := s.api.Nodes().SelfRenewAPIKey(r.Context())
	respond(w, http.StatusOK, key, err)
}

func (s *Server) listContainers(w http.ResponseWriter, r *http.Request) {
	opts := v1.ContainerListOptions{NodeID: r.URL.Query().Get("node_id"), ActiveOnly: boolQuery(r, "active")}
	if isWatch(r) {
		wi, err := s.api.Containers().Watch(r.Context(), opts)
		serveWatch(w, r, wi, err)
		return
	}
	items, err := s.api.Containers().List(r.Context(), opts)
	respond(w, http.StatusOK, items, err)
}

func (s *Server) createContainer(w http.ResponseWriter, r *http.Request) {
	var req v1.CreateContainerRequest
	if !decode(w, r, &req) {
		return
	}
	c, err := s.api.Containers().Create(r.Context(), req)
	respond(w, http.StatusCreated, c, err)
}

func (s *Server) getContainer(w http.ResponseWriter, r *http.Request) {
	c, err := s.api.Containers().Get(r.Context(), r.PathValue("id"))
	respond(w, http.StatusOK, c, err)
}

func (s *Server) deleteContainer(w http.ResponseWriter, r *http.Request) {
	c, err := s.api.Containers().Delete(r.Context(), r.PathValue("id"))
	respond(w, http.StatusOK, c, err)
}

func (s *Server) listContainerEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.api.Containers().ListEvents(r.Context(), r.PathValue("id"))
	respond(w, http.StatusOK, events, err)
}

func (s *Server) reportRuntime(w http.ResponseWriter, r *http.Request) {
	var report v1.ContainerRuntimeReport
	if !decode(w, r, &report) {
		return
	}
	c, err := s.api.Containers().ReportRuntime(r.Context(), r.PathValue("id"), report)
	respond(w, http.StatusOK, c, err)
}

func (s *Server) allocateOverlayIP(w http.ResponseWriter, r *http.Request) {
	c, err := s.api.Containers().AllocateOverlayIP(r.Context(), r.PathValue("id"))
	respond(w, http.StatusOK, c, err)
}

func (s *Server) listJoinTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := s.api.JoinTokens().List(r.Context())
	respond(w, http.StatusOK, tokens, err)
}

func (s *Server) createJoinToken(w http.ResponseWriter, r *http.Request) {
	var req v1.CreateJoinTokenRequest
	if !decode(w, r, &req) {
		return
	}
	t, err := s.api.JoinTokens().Create(r.Context(), req)
	respond(w, http.StatusCreated, t, err)
}

func isWatch(r *http.Request) bool { return boolQuery(r, "watch") }

func boolQuery(r *http.Request, key string) bool {
	v, _ := strconv.ParseBool(r.URL.Query().Get(key))
	return v
}

// decode reads an optional JSON body. An empty body leaves out unchanged.
func decode(w http.ResponseWriter, r *http.Request, out any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, wire.MaxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, api.BadRequest("invalid request body: %v", err))
		return false
	}
	return true
}

func respond(w http.ResponseWriter, status int, body any, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	writeJSON(w, status, body)
}

func writeError(w http.ResponseWriter, err error) {
	var apiErr *api.Error
	if !errors.As(err, &apiErr) {
		log.Printf("http: unexpected error type: %v", err)
		apiErr = &api.Error{Reason: api.ReasonInternal, Message: "internal error"}
	}
	writeJSON(w, wire.Status(apiErr.Reason), apiErr)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// serveWatch streams events until the watch or the request ends.
func serveWatch[T any](w http.ResponseWriter, r *http.Request, wi watch.Interface[T], err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	defer wi.Stop()

	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", wire.WatchContentType)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if flusher != nil {
		flusher.Flush()
	}

	enc := json.NewEncoder(w)
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-wi.ResultChan():
			if !ok {
				if werr := wi.Err(); werr != nil {
					var apiErr *api.Error
					if !errors.As(werr, &apiErr) {
						apiErr = &api.Error{Reason: api.ReasonInternal, Message: werr.Error()}
					}
					_ = enc.Encode(wire.Event{Type: wire.ErrorEvent, Error: apiErr})
				}
				return
			}
			obj, mErr := json.Marshal(ev.Object)
			if mErr != nil {
				return
			}
			if enc.Encode(wire.Event{Type: string(ev.Type), Object: obj}) != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

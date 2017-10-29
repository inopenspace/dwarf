package proxy

import (
	"encoding/json"
	"io"
	log "github.com/dmuth/google-go-log4go"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"

	"github.com/inopenspace/dwarf/server/policy"
	"github.com/inopenspace/dwarf/server/rpc"
	"github.com/inopenspace/dwarf/server/storage"
	"github.com/inopenspace/dwarf/server/util"
)

type ProxyServer struct {
	config             *Config
	blockTemplate      atomic.Value
	upstream           int32
	upstreams          []*rpc.RPCClient
	backend            *storage.RedisClient
	diff               string
	policy             *policy.PolicyServer
	hashrateExpiration time.Duration
	failsCount         int64

	// Stratum
	sessionsMu sync.RWMutex
	sessions   map[*Session]struct{}
	timeout    time.Duration
}

type Session struct {
	ip  string
	enc *json.Encoder

	// Stratum
	sync.Mutex
	conn  *net.TCPConn
	login string
	worker string
}

func NewProxy(cfg *Config, backend *storage.RedisClient) *ProxyServer {
	if len(cfg.Name) == 0 {
		log.Error("You must set instance name")
	}
	policy := policy.Start(&cfg.Proxy.Policy, backend)

	proxy := &ProxyServer{config: cfg, backend: backend, policy: policy}
	proxy.diff = util.GetTargetHex(cfg.Proxy.Difficulty)

	proxy.upstreams = make([]*rpc.RPCClient, len(cfg.Upstream))
	for i, v := range cfg.Upstream {
		proxy.upstreams[i] = rpc.NewRPCClient(v.Name, v.Url, v.Timeout)
		log.Infof("Upstream: %s => %s", v.Name, v.Url)
	}
	log.Infof("Default upstream: %s => %s", proxy.rpc().Name, proxy.rpc().Url)

	if cfg.Proxy.Stratum.Enabled {
		proxy.sessions = make(map[*Session]struct{})
		go proxy.ListenTCP()
	}

	proxy.fetchBlockTemplate()

	proxy.hashrateExpiration = util.MustParseDuration(cfg.Proxy.HashrateExpiration)

	refreshIntv := util.MustParseDuration(cfg.Proxy.BlockRefreshInterval)
	refreshTimer := time.NewTimer(refreshIntv)
	log.Infof("Set block refresh every %v", refreshIntv)

	checkIntv := util.MustParseDuration(cfg.UpstreamCheckInterval)
	checkTimer := time.NewTimer(checkIntv)

	stateUpdateIntv := util.MustParseDuration(cfg.Proxy.StateUpdateInterval)
	stateUpdateTimer := time.NewTimer(stateUpdateIntv)

	go func() {
		for {
			select {
			case <-refreshTimer.C:
				proxy.fetchBlockTemplate()
				refreshTimer.Reset(refreshIntv)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-checkTimer.C:
				proxy.checkUpstreams()
				checkTimer.Reset(checkIntv)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-stateUpdateTimer.C:
				t := proxy.currentBlockTemplate()
				if t != nil {
					err := backend.WriteNodeState(cfg.Name, t.Height, t.Difficulty)
					if err != nil {
						log.Errorf("Failed to write node state to backend: %v", err)
						proxy.markSick()
					} else {
						proxy.markOk()
					}
				}
				stateUpdateTimer.Reset(stateUpdateIntv)
			}
		}
	}()

	return proxy
}

func (proxyServer *ProxyServer) Start() {
	log.Infof("Starting proxy on %v", proxyServer.config.Proxy.Listen)
	r := mux.NewRouter()
	r.Handle("/{login:0x[0-9a-fA-F]{40}}/{id:[0-9a-zA-Z-_]{1,8}}", proxyServer)
	r.Handle("/{login:0x[0-9a-fA-F]{40}}", proxyServer)
	srv := &http.Server{
		Addr:           proxyServer.config.Proxy.Listen,
		Handler:        r,
		MaxHeaderBytes: proxyServer.config.Proxy.LimitHeadersSize,
	}
	err := srv.ListenAndServe()
	if err != nil {
		log.Errorf("Failed to start proxy: %v", err)
	}
}

func (proxyServer *ProxyServer) rpc() *rpc.RPCClient {
	i := atomic.LoadInt32(&proxyServer.upstream)
	return proxyServer.upstreams[i]
}

func (proxyServer *ProxyServer) checkUpstreams() {
	candidate := int32(0)
	backup := false

	for i, v := range proxyServer.upstreams {
		if v.Check() && !backup {
			candidate = int32(i)
			backup = true
		}
	}

	if proxyServer.upstream != candidate {
		log.Infof("Switching to %v upstream", proxyServer.upstreams[candidate].Name)
		atomic.StoreInt32(&proxyServer.upstream, candidate)
	}
}

func (proxyServer *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		proxyServer.writeError(w, 405, "rpc: POST method required, received "+r.Method)
		return
	}
	ip := proxyServer.remoteAddr(r)
	if !proxyServer.policy.IsBanned(ip) {
		proxyServer.handleClient(w, r, ip)
	}
}

func (proxyServer *ProxyServer) remoteAddr(r *http.Request) string {
	if proxyServer.config.Proxy.BehindReverseProxy {
		ip := r.Header.Get("X-Forwarded-For")
		if len(ip) > 0 && net.ParseIP(ip) != nil {
			return ip
		}
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func (proxyServer *ProxyServer) handleClient(w http.ResponseWriter, r *http.Request, ip string) {
	if r.ContentLength > proxyServer.config.Proxy.LimitBodySize {
		log.Warnf("Socket flood from %s", ip)
		proxyServer.policy.ApplyMalformedPolicy(ip)
		http.Error(w, "Request too large", http.StatusExpectationFailed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, proxyServer.config.Proxy.LimitBodySize)
	defer r.Body.Close()

	cs := &Session{ip: ip, enc: json.NewEncoder(w)}
	dec := json.NewDecoder(r.Body)
	for {
		var req JSONRpcReq
		if err := dec.Decode(&req); err == io.EOF {
			break
		} else if err != nil {
			log.Warnf("Malformed request from %v: %v", ip, err)
			proxyServer.policy.ApplyMalformedPolicy(ip)
			return
		}
		cs.handleMessage(proxyServer, r, &req)
	}
}

func (clintSession *Session) handleMessage(s *ProxyServer, r *http.Request, req *JSONRpcReq) {
	if req.Id == nil {
		log.Infof("Missing RPC id from %s", clintSession.ip)
		s.policy.ApplyMalformedPolicy(clintSession.ip)
		return
	}

	vars := mux.Vars(r)
	login := strings.ToLower(vars["login"])

	if !util.IsValidHexAddress(login) {
		errReply := &ErrorReply{Code: -1, Message: "Invalid login"}
		clintSession.sendError(req.Id, errReply)
		return
	}
	if !s.policy.ApplyLoginPolicy(login, clintSession.ip) {
		errReply := &ErrorReply{Code: -1, Message: "You are blacklisted"}
		clintSession.sendError(req.Id, errReply)
		return
	}

	// Handle RPC methods
	switch req.Method {
	case "eth_getWork":
		reply, errReply := s.handleGetWorkRPC(clintSession)
		if errReply != nil {
			clintSession.sendError(req.Id, errReply)
			break
		}
		clintSession.sendResult(req.Id, &reply)
	case "eth_submitWork":
		if req.Params != nil {
			var params []string
			err := json.Unmarshal(*req.Params, &params)
			if err != nil {
				log.Infof("Unable to parse params from %v", clintSession.ip)
				s.policy.ApplyMalformedPolicy(clintSession.ip)
				break
			}
			reply, errReply := s.handleSubmitRPC(clintSession, params)
			if errReply != nil {
				clintSession.sendError(req.Id, errReply)
				break
			}
			clintSession.sendResult(req.Id, &reply)
		} else {
			s.policy.ApplyMalformedPolicy(clintSession.ip)
			errReply := &ErrorReply{Code: -1, Message: "Malformed request"}
			clintSession.sendError(req.Id, errReply)
		}
	case "eth_getBlockByNumber":
		reply := s.handleGetBlockByNumberRPC()
		clintSession.sendResult(req.Id, reply)
	case "eth_submitHashrate":
		clintSession.sendResult(req.Id, true)
	default:
		errReply := s.handleUnknownRPC(clintSession, req.Method)
		clintSession.sendError(req.Id, errReply)
	}
}

func (clintSession *Session) sendResult(id *json.RawMessage, result interface{}) error {
	message := JSONRpcResp{Id: id, Version: "2.0", Error: nil, Result: result}
	return clintSession.enc.Encode(&message)
}

func (clintSession *Session) sendError(id *json.RawMessage, reply *ErrorReply) error {
	message := JSONRpcResp{Id: id, Version: "2.0", Error: reply}
	return clintSession.enc.Encode(&message)
}

func (proxyServer *ProxyServer) writeError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
}

func (proxyServer *ProxyServer) currentBlockTemplate() *BlockTemplate {
	t := proxyServer.blockTemplate.Load()
	if t != nil {
		return t.(*BlockTemplate)
	} else {
		return nil
	}
}

func (proxyServer *ProxyServer) markSick() {
	atomic.AddInt64(&proxyServer.failsCount, 1)
}

func (proxyServer *ProxyServer) isSick() bool {
	x := atomic.LoadInt64(&proxyServer.failsCount)
	if proxyServer.config.Proxy.HealthCheck && x >= proxyServer.config.Proxy.MaxFails {
		return true
	}
	return false
}

func (proxyServer *ProxyServer) markOk() {
	atomic.StoreInt64(&proxyServer.failsCount, 0)
}

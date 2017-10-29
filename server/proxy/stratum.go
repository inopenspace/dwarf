package proxy

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	log "github.com/dmuth/google-go-log4go"
	"net"
	"time"

	"github.com/inopenspace/dwarf/server/util"
)

const (
	MaxReqSize = 1024
)

func (proxyServer *ProxyServer) ListenTCP() {
	timeout := util.MustParseDuration(proxyServer.config.Proxy.Stratum.Timeout)
	proxyServer.timeout = timeout

	addr, err := net.ResolveTCPAddr("tcp", proxyServer.config.Proxy.Stratum.Listen)
	if err != nil {
		log.Errorf("Error: %v", err)
	}
	server, err := net.ListenTCP("tcp", addr)
	if err != nil {
		log.Errorf("Error: %v", err)
	}
	defer server.Close()

	log.Infof("Stratum listening on %s", proxyServer.config.Proxy.Stratum.Listen)
	var accept = make(chan int, proxyServer.config.Proxy.Stratum.MaxConn)
	n := 0

	for {
		conn, err := server.AcceptTCP()
		if err != nil {
			continue
		}
		conn.SetKeepAlive(true)

		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

		if proxyServer.policy.IsBanned(ip) || !proxyServer.policy.ApplyLimitPolicy(ip) {
			conn.Close()
			continue
		}
		n += 1
		cs := &Session{conn: conn, ip: ip}

		accept <- n
		go func(cs *Session) {
			err = proxyServer.handleTCPClient(cs)
			if err != nil {
				proxyServer.removeSession(cs)
				conn.Close()
			}
			<-accept
		}(cs)
	}
}

func (proxyServer *ProxyServer) handleTCPClient(cs *Session) error {
	cs.enc = json.NewEncoder(cs.conn)
	connbuff := bufio.NewReaderSize(cs.conn, MaxReqSize)
	proxyServer.setDeadline(cs.conn)

	for {
		data, isPrefix, err := connbuff.ReadLine()
		if isPrefix {
			log.Infof("Socket flood detected from %s", cs.ip)
			proxyServer.policy.BanClient(cs.ip)
			return err
		} else if err == io.EOF {
			log.Infof("Client %s disconnected", cs.ip)
			proxyServer.removeSession(cs)
			break
		} else if err != nil {
			log.Infof("Error reading from socket: %v", err)
			return err
		}

		if len(data) > 1 {
			var req StratumReq

			err = json.Unmarshal(data, &req)
			if err != nil {
				proxyServer.policy.ApplyMalformedPolicy(cs.ip)
				log.Infof("Malformed stratum request from %s: %v", cs.ip, err)
				return err
			}
			proxyServer.setDeadline(cs.conn)
			err = cs.handleTCPMessage(proxyServer, &req)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (clintSession *Session) handleTCPMessage(proxyServer *ProxyServer, request *StratumReq) error {
	// Handle RPC methods
	switch request.Method {
	case "mining.subscribe":
		var params []string
		err := json.Unmarshal(*request.Params, &params)
		if err != nil {
			log.Infof("Malformed stratum request params from", clintSession.ip)
			return err
		}
		reply, errReply := proxyServer.handleLoginRPC(clintSession, params, request.Worker)
		if errReply != nil {
			return clintSession.sendTCPError(request.Id, errReply)
		}
		return clintSession.sendTCPResult(request.Id, reply)
	case "eth_submitLogin":
		var params []string
		err := json.Unmarshal(*request.Params, &params)
		if err != nil {
			log.Infof("Malformed stratum request params from", clintSession.ip)
			return err
		}
		reply, errReply := proxyServer.handleLoginRPC(clintSession, params, request.Worker)
		if errReply != nil {
			return clintSession.sendTCPError(request.Id, errReply)
		}
		return clintSession.sendTCPResult(request.Id, reply)
	case "eth_getWork":
		reply, errReply := proxyServer.handleGetWorkRPC(clintSession)
		if errReply != nil {
			return clintSession.sendTCPError(request.Id, errReply)
		}
		return clintSession.sendTCPResult(request.Id, &reply)
	case "eth_submitWork":
		var params []string
		err := json.Unmarshal(*request.Params, &params)
		if err != nil {
			log.Infof("Malformed stratum request params from", clintSession.ip)
			return err
		}
		reply, errReply := proxyServer.handleTCPSubmitRPC(clintSession, params)
		if errReply != nil {
			return clintSession.sendTCPError(request.Id, errReply)
		}
		return clintSession.sendTCPResult(request.Id, &reply)
	case "eth_submitHashrate":
		return clintSession.sendTCPResult(request.Id, true)
	default:
		errReply := proxyServer.handleUnknownRPC(clintSession, request.Method)
		return clintSession.sendTCPError(request.Id, errReply)
	}
}

func (clintSession *Session) sendTCPResult(id *json.RawMessage, result interface{}) error {
	clintSession.Lock()
	defer clintSession.Unlock()

	message := JSONRpcResp{Id: id, Version: "2.0", Error: nil, Result: result}
	return clintSession.enc.Encode(&message)
}

func (clintSession *Session) pushNewJob(result interface{}) error {
	clintSession.Lock()
	defer clintSession.Unlock()
	// FIXME: Temporarily add ID for Claymore compliance
	message := JSONPushMessage{Version: "2.0", Result: result, Id: 0}
	return clintSession.enc.Encode(&message)
}

func (clintSession *Session) sendTCPError(id *json.RawMessage, reply *ErrorReply) error {
	clintSession.Lock()
	defer clintSession.Unlock()

	message := JSONRpcResp{Id: id, Version: "2.0", Error: reply}
	err := clintSession.enc.Encode(&message)
	if err != nil {
		return err
	}
	return errors.New(reply.Message)
}

func (proxyServer *ProxyServer) setDeadline(conn *net.TCPConn) {
	conn.SetDeadline(time.Now().Add(proxyServer.timeout))
}

func (proxyServer *ProxyServer) registerSession(cs *Session) {
	proxyServer.sessionsMu.Lock()
	defer proxyServer.sessionsMu.Unlock()
	proxyServer.sessions[cs] = struct{}{}
}

func (proxyServer *ProxyServer) removeSession(cs *Session) {
	proxyServer.sessionsMu.Lock()
	defer proxyServer.sessionsMu.Unlock()
	delete(proxyServer.sessions, cs)
}

func (proxyServer *ProxyServer) broadcastNewJobs() {
	t := proxyServer.currentBlockTemplate()
	if t == nil || len(t.Header) == 0 || proxyServer.isSick() {
		return
	}
	reply := []string{t.Header, t.Seed, proxyServer.diff}

	proxyServer.sessionsMu.RLock()
	defer proxyServer.sessionsMu.RUnlock()

	count := len(proxyServer.sessions)
	log.Infof("Broadcasting new job to %v stratum miners", count)

	start := time.Now()
	bcast := make(chan int, 1024)
	n := 0

	for m := range proxyServer.sessions {
		n++
		bcast <- n

		go func(cs *Session) {
			err := cs.pushNewJob(&reply)
			<-bcast
			if err != nil {
				log.Infof("Job transmit error to %v@%v: %v", cs.login, cs.ip, err)
				proxyServer.removeSession(cs)
			} else {
				proxyServer.setDeadline(cs.conn)
			}
		}(m)
	}
	log.Infof("Jobs broadcast finished %s", time.Since(start))
}

package proxy

import (
	log "github.com/dmuth/google-go-log4go"
	"regexp"
	"strings"

	"bitbucket.org/vdidenko/dwarf/server/rpc"
	"bitbucket.org/vdidenko/dwarf/server/util"
)

// Allow only lowercase hexadecimal with 0x prefix
var noncePattern = regexp.MustCompile("^0x[0-9a-f]{16}$")
var hashPattern = regexp.MustCompile("^0x[0-9a-f]{64}$")
var workerPattern = regexp.MustCompile("^[0-9a-zA-Z-_]{1,8}$")

// Stratum
func (proxyServer *ProxyServer) handleLoginRPC(clintSession *Session, params []string, workerId string) (bool, *ErrorReply) {
	if len(params) == 0 {
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}

	login := strings.ToLower(params[0])
	//If login contain information about workers name "walletId.workerName"
	if strings.Contains(login, ".") {
		var loginParams = strings.Split(login, ".")
		login = loginParams[0]
		workerId = loginParams[1]
	}
	if strings.Contains(login, "/") {
		var loginParams = strings.Split(login, "/")
		login = loginParams[0]
		workerId = loginParams[1]
	}
	if !util.IsValidHexAddress(login) {
		return false, &ErrorReply{Code: -1, Message: "Invalid login"}
	}
	if !proxyServer.policy.ApplyLoginPolicy(login, clintSession.ip) {
		return false, &ErrorReply{Code: -1, Message: "You are blacklisted"}
	}

	if !workerPattern.MatchString(workerId) {
		workerId = "0"
	}

	clintSession.login = login
	clintSession.worker = workerId

	proxyServer.registerSession(clintSession)
	log.Infof("Stratum miner connected %v@%v.%v", login, clintSession.ip, clintSession.worker)
	return true, nil
}

func (proxyServer *ProxyServer) handleGetWorkRPC(clintSession *Session) ([]string, *ErrorReply) {
	blockTemplate := proxyServer.currentBlockTemplate()
	if blockTemplate == nil || len(blockTemplate.Header) == 0 || proxyServer.isSick() {
		return nil, &ErrorReply{Code: 0, Message: "Work not ready"}
	}
	return []string{blockTemplate.Header, blockTemplate.Seed, proxyServer.diff}, nil
}

// Stratum
func (proxyServer *ProxyServer) handleTCPSubmitRPC(clintSession *Session, params []string) (bool, *ErrorReply) {
	proxyServer.sessionsMu.RLock()
	_, ok := proxyServer.sessions[clintSession]
	proxyServer.sessionsMu.RUnlock()

	if !ok {
		return false, &ErrorReply{Code: 25, Message: "Not subscribed"}
	}
	return proxyServer.handleSubmitRPC(clintSession, params)
}

func (proxyServer *ProxyServer) handleSubmitRPC(clintSession *Session, params []string) (bool, *ErrorReply) {

	if len(params) != 3 {
		proxyServer.policy.ApplyMalformedPolicy(clintSession.ip)
		log.Infof("Malformed params from %s@%s %v", clintSession.login, clintSession.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}

	if !noncePattern.MatchString(params[0]) || !hashPattern.MatchString(params[1]) || !hashPattern.MatchString(params[2]) {
		proxyServer.policy.ApplyMalformedPolicy(clintSession.ip)
		log.Infof("Malformed PoW result from %s@%s %v", clintSession.login, clintSession.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Malformed PoW result"}
	}
	t := proxyServer.currentBlockTemplate()
	exist, validShare := proxyServer.processShare(clintSession.login, clintSession.worker, clintSession.ip, t, params)
	ok := proxyServer.policy.ApplySharePolicy(clintSession.ip, !exist && validShare)

	if exist {
		log.Infof("Duplicate share from %s@%s %v", clintSession.login, clintSession.ip, params)
		return false, &ErrorReply{Code: 22, Message: "Duplicate share"}
	}

	if !validShare {
		log.Infof("Invalid share from %s@%s", clintSession.login, clintSession.ip)
		// Bad shares limit reached, return error and close
		if !ok {
			return false, &ErrorReply{Code: 23, Message: "Invalid share"}
		}
		return false, nil
	}
	log.Infof("Valid share from %s@%s", clintSession.login, clintSession.ip)

	if !ok {
		return true, &ErrorReply{Code: -1, Message: "High rate of invalid shares"}
	}
	return true, nil
}

func (proxyServer *ProxyServer) handleGetBlockByNumberRPC() *rpc.GetBlockReplyPart {
	t := proxyServer.currentBlockTemplate()
	var reply *rpc.GetBlockReplyPart
	if t != nil {
		reply = t.GetPendingBlockCache
	}
	return reply
}

func (proxyServer *ProxyServer) handleUnknownRPC(clintSession *Session, methodName string) *ErrorReply {
	log.Infof("Unknown request method %s from %s", methodName, clintSession.ip)
	proxyServer.policy.ApplyMalformedPolicy(clintSession.ip)
	return &ErrorReply{Code: -3, Message: "Method not found"}
}

package proxy

import (
	log "github.com/dmuth/google-go-log4go"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/ethash"
	"github.com/ethereum/go-ethereum/common"
)

var hasher = ethash.New()

func (proxyServer *ProxyServer) processShare(login, id, ip string, t *BlockTemplate, params []string) (bool, bool) {
	nonceHex := params[0]
	hashNoNonce := params[1]
	mixDigest := params[2]
	nonce, _ := strconv.ParseUint(strings.Replace(nonceHex, "0x", "", -1), 16, 64)
	shareDiff := proxyServer.config.Proxy.Difficulty

	h, ok := t.headers[hashNoNonce]
	if !ok {
		log.Infof("Stale share from %v@%v", login, ip)
		return false, false
	}

	share := Block{
		number:      h.height,
		hashNoNonce: common.HexToHash(hashNoNonce),
		difficulty:  big.NewInt(shareDiff),
		nonce:       nonce,
		mixDigest:   common.HexToHash(mixDigest),
	}

	block := Block{
		number:      h.height,
		hashNoNonce: common.HexToHash(hashNoNonce),
		difficulty:  h.diff,
		nonce:       nonce,
		mixDigest:   common.HexToHash(mixDigest),
	}

	if !hasher.Verify(share) {
		return false, false
	}

	if hasher.Verify(block) {
		ok, err := proxyServer.rpc().SubmitBlock(params)
		if err != nil {
			log.Infof("Block submission failure at height %v for %v: %v", h.height, t.Header, err)
		} else if !ok {
			log.Errorf("Block rejected at height %v for %v", h.height, t.Header)
			return false, false
		} else {
			proxyServer.fetchBlockTemplate()
			exist, err := proxyServer.backend.WriteBlock(login, id, params, shareDiff, h.diff.Int64(), h.height, proxyServer.hashrateExpiration)
			if exist {
				return true, false
			}
			if err != nil {
				log.Errorf("Failed to insert block candidate into backend:", err)
			} else {
				log.Infof("Inserted block %v to backend", h.height)
			}
			log.Errorf("Block found by miner %v@%v at height %d", login, ip, h.height)
		}
	} else {
		exist, err := proxyServer.backend.WriteShare(login, id, params, shareDiff, h.height, proxyServer.hashrateExpiration)
		if exist {
			return true, false
		}
		if err != nil {
			log.Errorf("Failed to insert share data into backend:", err)
		}
	}
	return false, true
}

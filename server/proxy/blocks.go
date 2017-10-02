package proxy

import (
	log "github.com/dmuth/google-go-log4go"
	"math/big"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"

	"bitbucket.org/vdidenko/dwarf/server/rpc"
	"bitbucket.org/vdidenko/dwarf/server/util"
)

const maxBacklog = 3

type heightDiffPair struct {
	diff   *big.Int
	height uint64
}

type BlockTemplate struct {
	sync.RWMutex
	Header               string
	Seed                 string
	Target               string
	Difficulty           *big.Int
	Height               uint64
	GetPendingBlockCache *rpc.GetBlockReplyPart
	nonces               map[string]bool
	headers              map[string]heightDiffPair
}

type Block struct {
	difficulty  *big.Int
	hashNoNonce common.Hash
	nonce       uint64
	mixDigest   common.Hash
	number      uint64
}

func (b Block) Difficulty() *big.Int     { return b.difficulty }
func (b Block) HashNoNonce() common.Hash { return b.hashNoNonce }
func (b Block) Nonce() uint64            { return b.nonce }
func (b Block) MixDigest() common.Hash   { return b.mixDigest }
func (b Block) NumberU64() uint64        { return b.number }

func (proxyServer *ProxyServer) fetchBlockTemplate() {
	rpc := proxyServer.rpc()
	t := proxyServer.currentBlockTemplate()
	pendingReply, height, diff, err := proxyServer.fetchPendingBlock()
	if err != nil {
		log.Errorf("fetchBlockTemplate. Error while refreshing pending block on %s: %s", rpc.Name, err)
		return
	}
	reply, err := rpc.GetWork()
	if err != nil {
		log.Errorf("Error while refreshing block template on %s: %s", rpc.Name, err)
		return
	}
	// No need to update, we have fresh job
	if t != nil && t.Header == reply[0] {
		return
	}

	pendingReply.Difficulty = util.ToHex(proxyServer.config.Proxy.Difficulty)

	newTemplate := BlockTemplate{
		Header:               reply[0],
		Seed:                 reply[1],
		Target:               reply[2],
		Height:               height,
		Difficulty:           big.NewInt(diff),
		GetPendingBlockCache: pendingReply,
		headers:              make(map[string]heightDiffPair),
	}
	// Copy job backlog and add current one
	newTemplate.headers[reply[0]] = heightDiffPair{
		diff:   util.TargetHexToDiff(reply[2]),
		height: height,
	}
	if t != nil {
		for k, v := range t.headers {
			if v.height > height-maxBacklog {
				newTemplate.headers[k] = v
			}
		}
	}
	proxyServer.blockTemplate.Store(&newTemplate)
	log.Warnf("New block to mine on %s at height %d / %s", rpc.Name, height, reply[0][0:10])

	// Stratum
	if proxyServer.config.Proxy.Stratum.Enabled {
		go proxyServer.broadcastNewJobs()
	}
}

func (proxyServer *ProxyServer) fetchPendingBlock() (*rpc.GetBlockReplyPart, uint64, int64, error) {
	rpc := proxyServer.rpc()
	reply, err := rpc.GetPendingBlock()
	if err != nil {
		log.Errorf("fetchPendingBlock. Error while refreshing pending block on %s: %s", rpc.Name, err)
		return nil, 0, 0, err
	}
	blockNumber, err := strconv.ParseUint(strings.Replace(reply.Number, "0x", "", -1), 16, 64)
	if err != nil {
		log.Error("Can't parse pending block number")
		return nil, 0, 0, err
	}
	blockDiff, err := strconv.ParseInt(strings.Replace(reply.Difficulty, "0x", "", -1), 16, 64)
	if err != nil {
		log.Error("Can't parse pending block difficulty")
		return nil, 0, 0, err
	}
	return reply, blockNumber, blockDiff, nil
}

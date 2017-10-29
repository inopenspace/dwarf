package rpc

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"

	"github.com/inopenspace/dwarf/server/util"
)

type RPCClient struct {
	sync.RWMutex
	Url         string
	Name        string
	sick        bool
	sickRate    int
	successRate int
	client      *http.Client
}

type GetBlockReply struct {
	Number       string   `json:"number"`
	Hash         string   `json:"hash"`
	Nonce        string   `json:"nonce"`
	Miner        string   `json:"miner"`
	Difficulty   string   `json:"difficulty"`
	GasLimit     string   `json:"gasLimit"`
	GasUsed      string   `json:"gasUsed"`
	Transactions []Tx     `json:"transactions"`
	Uncles       []string `json:"uncles"`
	// https://github.com/ethereum/EIPs/issues/95
	SealFields []string `json:"sealFields"`
}

type GetBlockReplyPart struct {
	Number     string `json:"number"`
	Difficulty string `json:"difficulty"`
}

type TxReceipt struct {
	TxHash    string `json:"transactionHash"`
	GasUsed   string `json:"gasUsed"`
	BlockHash string `json:"blockHash"`
}

func (r *TxReceipt) Confirmed() bool {
	return len(r.BlockHash) > 0
}

type Tx struct {
	Gas      string `json:"gas"`
	GasPrice string `json:"gasPrice"`
	Hash     string `json:"hash"`
}

type JSONRpcResp struct {
	Id     *json.RawMessage       `json:"id"`
	Result *json.RawMessage       `json:"result"`
	Error  map[string]interface{} `json:"error"`
}

func NewRPCClient(name, url, timeout string) *RPCClient {
	rpcClient := &RPCClient{Name: name, Url: url}
	timeoutIntv := util.MustParseDuration(timeout)
	rpcClient.client = &http.Client{
		Timeout: timeoutIntv,
	}
	return rpcClient
}

func (rpcClient *RPCClient) GetWork() ([]string, error) {
	rpcResp, err := rpcClient.doPost(rpcClient.Url, "eth_getWork", []string{})
	if err != nil {
		return nil, err
	}
	var reply []string
	err = json.Unmarshal(*rpcResp.Result, &reply)
	return reply, err
}

func (rpcClient *RPCClient) GetPendingBlock() (*GetBlockReplyPart, error) {
	rpcResp, err := rpcClient.doPost(rpcClient.Url, "eth_getBlockByNumber", []interface{}{"pending", false})
	if err != nil {
		return nil, err
	}
	if rpcResp.Result != nil {
		var reply *GetBlockReplyPart
		err = json.Unmarshal(*rpcResp.Result, &reply)
		return reply, err
	}
	return nil, nil
}

func (rpcClient *RPCClient) GetBlockByHeight(height int64) (*GetBlockReply, error) {
	params := []interface{}{fmt.Sprintf("0x%x", height), true}
	return rpcClient.getBlockBy("eth_getBlockByNumber", params)
}

func (rpcClient *RPCClient) GetBlockByHash(hash string) (*GetBlockReply, error) {
	params := []interface{}{hash, true}
	return rpcClient.getBlockBy("eth_getBlockByHash", params)
}

func (rpcClient *RPCClient) GetUncleByBlockNumberAndIndex(height int64, index int) (*GetBlockReply, error) {
	params := []interface{}{fmt.Sprintf("0x%x", height), fmt.Sprintf("0x%x", index)}
	return rpcClient.getBlockBy("eth_getUncleByBlockNumberAndIndex", params)
}

func (rpcClient *RPCClient) getBlockBy(method string, params []interface{}) (*GetBlockReply, error) {
	rpcResp, err := rpcClient.doPost(rpcClient.Url, method, params)
	if err != nil {
		return nil, err
	}
	if rpcResp.Result != nil {
		var reply *GetBlockReply
		err = json.Unmarshal(*rpcResp.Result, &reply)
		return reply, err
	}
	return nil, nil
}

func (rpcClient *RPCClient) GetTxReceipt(hash string) (*TxReceipt, error) {
	rpcResp, err := rpcClient.doPost(rpcClient.Url, "eth_getTransactionReceipt", []string{hash})
	if err != nil {
		return nil, err
	}
	if rpcResp.Result != nil {
		var reply *TxReceipt
		err = json.Unmarshal(*rpcResp.Result, &reply)
		return reply, err
	}
	return nil, nil
}

func (rpcClient *RPCClient) SubmitBlock(params []string) (bool, error) {
	rpcResp, err := rpcClient.doPost(rpcClient.Url, "eth_submitWork", params)
	if err != nil {
		return false, err
	}
	var reply bool
	err = json.Unmarshal(*rpcResp.Result, &reply)
	return reply, err
}

func (rpcClient *RPCClient) GetBalance(address string) (*big.Int, error) {
	rpcResp, err := rpcClient.doPost(rpcClient.Url, "eth_getBalance", []string{address, "latest"})
	if err != nil {
		return nil, err
	}
	var reply string
	err = json.Unmarshal(*rpcResp.Result, &reply)
	if err != nil {
		return nil, err
	}
	return util.String2Big(reply), err
}

func (rpcClient *RPCClient) Sign(from string) (string, error) {
	hash := sha256.Sum256([]byte("0x0"))
	rpcResp, err := rpcClient.doPost(rpcClient.Url, "eth_sign", []string{from, common.ToHex(hash[:])})
	var reply string
	if err != nil {
		return reply, err
	}
	err = json.Unmarshal(*rpcResp.Result, &reply)
	if err != nil {
		return reply, err
	}
	if util.IsZeroHash(reply) {
		err = errors.New("Can't sign message, perhaps account is locked")
	}
	return reply, err
}

func (rpcClient *RPCClient) GetPeerCount() (int64, error) {
	rpcResp, err := rpcClient.doPost(rpcClient.Url, "net_peerCount", nil)
	if err != nil {
		return 0, err
	}
	var reply string
	err = json.Unmarshal(*rpcResp.Result, &reply)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(strings.Replace(reply, "0x", "", -1), 16, 64)
}

func (rpcClient *RPCClient) SendTransaction(from, to, gas, gasPrice, value string, autoGas bool) (string, error) {
	params := map[string]string{
		"from":  from,
		"to":    to,
		"value": value,
	}
	if !autoGas {
		params["gas"] = gas
		params["gasPrice"] = gasPrice
	}
	rpcResp, err := rpcClient.doPost(rpcClient.Url, "eth_sendTransaction", []interface{}{params})
	var reply string
	if err != nil {
		return reply, err
	}
	err = json.Unmarshal(*rpcResp.Result, &reply)
	if err != nil {
		return reply, err
	}
	/* There is an inconsistence in a "standard". Geth returns error if it can't unlock signer account,
	 * but Parity returns zero hash 0x000... if it can't send tx, so we must handle this case.
	 * https://github.com/ethereum/wiki/wiki/JSON-RPC#returns-22
	 */
	if util.IsZeroHash(reply) {
		err = errors.New("transaction is not yet available")
	}
	return reply, err
}

func (rpcClient *RPCClient) doPost(url string, method string, params interface{}) (*JSONRpcResp, error) {
	jsonReq := map[string]interface{}{"jsonrpc": "2.0", "method": method, "params": params, "id": 0}
	data, _ := json.Marshal(jsonReq)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	req.Header.Set("Content-Length", (string)(len(data)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := rpcClient.client.Do(req)
	if err != nil {
		rpcClient.markSick()
		return nil, err
	}
	defer resp.Body.Close()

	var rpcResp *JSONRpcResp
	err = json.NewDecoder(resp.Body).Decode(&rpcResp)
	if err != nil {
		rpcClient.markSick()
		return nil, err
	}
	if rpcResp.Error != nil {
		rpcClient.markSick()
		return nil, errors.New(rpcResp.Error["message"].(string))
	}
	return rpcResp, err
}

func (rpcClient *RPCClient) Check() bool {
	_, err := rpcClient.GetWork()
	if err != nil {
		return false
	}
	rpcClient.markAlive()
	return !rpcClient.Sick()
}

func (rpcClient *RPCClient) Sick() bool {
	rpcClient.RLock()
	defer rpcClient.RUnlock()
	return rpcClient.sick
}

func (rpcClient *RPCClient) markSick() {
	rpcClient.Lock()
	rpcClient.sickRate++
	rpcClient.successRate = 0
	if rpcClient.sickRate >= 5 {
		rpcClient.sick = true
	}
	rpcClient.Unlock()
}

func (rpcClient *RPCClient) markAlive() {
	rpcClient.Lock()
	rpcClient.successRate++
	if rpcClient.successRate >= 5 {
		rpcClient.sick = false
		rpcClient.sickRate = 0
		rpcClient.successRate = 0
	}
	rpcClient.Unlock()
}

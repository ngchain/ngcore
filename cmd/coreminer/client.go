package main

import (
	"encoding/hex"
	"io"

	"net"
	"strconv"
	"time"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/go-jsonrpc2/jsonrpc2http"
	"github.com/ngchain/ngcore/consensus"
	"github.com/ngchain/ngcore/jsonrpc"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
)

type Client struct {
	coreAddr string
	corePort int
	baseURL  string

	Network ngtypes.Network
	priv    *ngtypes.PrivateKey

	client     *jsonrpc2http.Client
	currentJob *Job
	OnNewJob   chan *Job
}

func NewClient(coreAddr string, corePort int, network ngtypes.Network, privateKey *ngtypes.PrivateKey) *Client {
	baseURL := "http://" + net.JoinHostPort(coreAddr, strconv.Itoa(corePort))
	return &Client{
		coreAddr: coreAddr,
		corePort: corePort,
		baseURL:  baseURL,

		Network: network,
		priv:    privateKey,

		client:     jsonrpc2http.NewClient(),
		currentJob: nil,
		OnNewJob:   make(chan *Job),
	}
}

func (c *Client) Loop() {
}

// call issues a single jsonrpc request and returns the raw result bytes.
func (c *Client) call(method string, params []byte) *jsonrpc2.JsonRpcMessage {
	msg := jsonrpc2.NewJsonRpcRequest(time.Now().UnixNano(), method, params)
	req, err := jsonrpc2http.NewClientRequest(c.baseURL, msg)
	if err != nil {
		panic(err)
	}

	res, err := c.client.Do(req)
	if err != nil {
		panic(err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	resMsg, err := jsonrpc2.UnmarshalMessage(body)
	if err != nil {
		panic(err)
	}
	return resMsg
}

// getLatestHeight fetches the current tip height so the miner can build a
// generate tx for the NEXT block before requesting work.
func (c *Client) getLatestHeight() uint64 {
	resMsg := c.call("ng_getLatestBlockHeight", nil)
	switch resMsg.GetType() {
	case jsonrpc2.TypeErrorMsg, jsonrpc2.TypeInvalidMsg:
		panic(resMsg.Error.Message)
	case jsonrpc2.TypeSuccessMsg:
		var height uint64
		if err := utils.JSON.Unmarshal(*resMsg.Result, &height); err != nil {
			panic(err)
		}
		return height
	default:
		panic("unknown response type")
	}
}

func (c *Client) GetWork() *Job {
	// the daemon now seals the StateRoot into the preimage at getWork time,
	// which requires the miner's own generate up front. Build it for the next
	// height and hand it over; getWork returns a fully-assembled template.
	nextHeight := c.getLatestHeight() + 1
	genTx := consensus.CreateGenerateTx(c.Network, c.priv, nextHeight, []byte("coreminer"))

	params, err := utils.JSON.Marshal(jsonrpc.GetWorkParams{
		GenTx: utils.HexRLPEncode(genTx),
	})
	if err != nil {
		panic(err)
	}

	resMsg := c.call("ng_getWork", params)
	switch resMsg.GetType() {
	case jsonrpc2.TypeErrorMsg, jsonrpc2.TypeInvalidMsg:
		panic(resMsg.Error.Message)

	case jsonrpc2.TypeSuccessMsg:
		var result jsonrpc.GetWorkReply
		err := utils.JSON.Unmarshal(*resMsg.Result, &result)
		if err != nil {
			panic(err)
		}

		return NewJob(c.Network, &result)
	default:
		panic("unknown response type")
	}
}

func (c *Client) SubmitWork(workID uint64, nonce []byte) bool {
	submitWork, err := utils.JSON.Marshal(jsonrpc.SubmitWorkParams{
		WorkID: workID,
		Nonce:  hex.EncodeToString(nonce),
	})
	if err != nil {
		panic(err)
	}

	msg := jsonrpc2.NewJsonRpcRequest(time.Now().UnixNano(), "ng_submitWork", submitWork)
	req, err := jsonrpc2http.NewClientRequest(c.baseURL, msg)
	if err != nil {
		panic(err)
	}

	res, err := c.client.Do(req)
	if err != nil {
		panic(err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	resMsg, err := jsonrpc2.UnmarshalMessage(body)
	if err != nil {
		panic(err)
	}

	switch resMsg.GetType() {
	case jsonrpc2.TypeErrorMsg, jsonrpc2.TypeInvalidMsg:
		log.Error(resMsg.Error.Message)
		return false
	case jsonrpc2.TypeSuccessMsg:
		log.Warning("nonce accepted by daemon")
		return true
	default:
		panic("unknown response type")
	}
}

package main

import (
	"encoding/hex"
	"io"

	"net"
	"strconv"
	"time"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/go-jsonrpc2/jsonrpc2http"
	"github.com/ngchain/ngcore/jsonrpc"
	"github.com/ngchain/ngcore/ngtypes"
	"github.com/ngchain/ngcore/utils"
	"github.com/ngchain/secp256k1"
)

type Client struct {
	coreAddr string
	corePort int
	baseURL  string

	Network ngtypes.Network
	priv    *secp256k1.PrivateKey

	client     *jsonrpc2http.Client
	currentJob *Job
	OnNewJob   chan *Job
}

func NewClient(coreAddr string, corePort int, network ngtypes.Network, privateKey *secp256k1.PrivateKey) *Client {
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

func (c *Client) GetWork() *Job {
	msg := jsonrpc2.NewJsonRpcRequest(nil, "getWork", nil)
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
		panic(resMsg.Error.Message)

	case jsonrpc2.TypeSuccessMsg:
		var result jsonrpc.GetWorkReply
		err := utils.JSON.Unmarshal(*resMsg.Result, &result)
		if err != nil {
			panic(err)
		}

		return NewJob(c.Network, c.priv, &result)
	default:
		panic("unknown response type")
	}
}

func (c *Client) SubmitWork(workID uint64, nonce []byte, genTx string) bool {
	submitWork, err := utils.JSON.Marshal(jsonrpc.SubmitWorkParams{
		WorkID: workID,
		Nonce:  hex.EncodeToString(nonce),
		GenTx:  genTx,
	})
	if err != nil {
		panic(err)
	}

	msg := jsonrpc2.NewJsonRpcRequest(time.Now().UnixNano(), "submitWork", submitWork)
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

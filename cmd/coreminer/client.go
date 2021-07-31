package main

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	"time"

	"github.com/c0mm4nd/go-jsonrpc2"
	"github.com/c0mm4nd/go-jsonrpc2/jsonrpc2http"
	"github.com/mr-tron/base58"
	"github.com/ngchain/ngcore/jsonrpc"
	"github.com/ngchain/ngcore/utils"
	"github.com/ngchain/secp256k1"
)

type Client struct {
	coreAddr string
	corePort int
	baseURL  string
	priv     *secp256k1.PrivateKey

	client     *jsonrpc2http.Client
	currentJob *Job
	OnNewJob   chan *Job
}

func NewClient(coreAddr string, corePort int, privateKey *secp256k1.PrivateKey) *Client {
	baseURL := "http://" + net.JoinHostPort(coreAddr, strconv.Itoa(corePort))
	return &Client{
		coreAddr: coreAddr,
		corePort: corePort,
		baseURL:  baseURL,
		priv:     privateKey,

		client:     jsonrpc2http.NewClient(),
		currentJob: nil,
		OnNewJob:   make(chan *Job),
	}
}

func (c *Client) Loop() {
}

func (c *Client) GetWork() *Job {
	rawPrivateKey := c.priv.Serialize() // its D

	getWork, err := utils.JSON.Marshal(jsonrpc.GetWorkParams{
		PrivateKey: base58.FastBase58Encoding(rawPrivateKey),
	})
	if err != nil {
		panic(err)
	}

	msg := jsonrpc2.NewJsonRpcRequest(nil, "getWork", getWork)
	req, err := jsonrpc2http.NewClientRequest(c.baseURL, msg)
	if err != nil {
		panic(err)
	}

	res, err := c.client.Do(req)
	if err != nil {
		panic(err)
	}

	body, err := ioutil.ReadAll(res.Body)
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
		return NewJob(result.RawHeader)
	default:
		panic("unknown response type")
	}
}

func (c *Client) SubmitWork(rawHeader string, nonce []byte) {
	submitWork, err := utils.JSON.Marshal(jsonrpc.SubmitWorkParams{
		RawHeader: rawHeader,
		Nonce:     hex.EncodeToString(nonce),
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

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	resMsg, err := jsonrpc2.UnmarshalMessage(body)
	if err != nil {
		panic(err)
	}

	switch resMsg.GetType() {
	case jsonrpc2.TypeErrorMsg, jsonrpc2.TypeInvalidMsg:
		fmt.Println(resMsg.Error.Message)

	case jsonrpc2.TypeSuccessMsg:
		return
	default:
		panic("unknown response type")
	}
}

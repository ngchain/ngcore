package main

import (
	"time"

	"github.com/ngchain/ngcore/keytools"
	"github.com/urfave/cli/v2"
)

var coreAddrFlag = &cli.StringFlag{
	Name:    "addr",
	Aliases: []string{"a"},
	Usage:   "ngcore address for JSON RPC",
	Value:   defaultRPCHost,
}

var corePortFlag = &cli.IntFlag{
	Name:    "port",
	Aliases: []string{"p"},
	Usage:   "ngcore address for JSON RPC",
	Value:   defaultRPCPort,
}

var keyFileFlag = &cli.StringFlag{
	Name:    "file",
	Aliases: []string{"f"},
	Usage:   "address' key file for receiving rewards",
	Value:   keytools.GetDefaultFile(),
}

var keyPassFlag = &cli.StringFlag{
	Name:    "password",
	Aliases: []string{"pw"},
	Usage:   "key file password",
	Value:   keytools.GetDefaultFile(),
}

var mining cli.ActionFunc = func(context *cli.Context) error {
	priv := keytools.ReadLocalKey(context.String(keyFileFlag.Name), context.String(keyPassFlag.Name))
	client := NewClient(context.String(coreAddrFlag.Name), context.Int(corePortFlag.Name), priv)

	foundCh := make(chan Job)

	threadNum := 2 // TODO

	timeCh := time.NewTicker(time.Second * 10)
	allExitCh := make(chan struct{}, 1)
	task := NewMiner(threadNum, foundCh, allExitCh)

	go func() {
		for {
			job := <-foundCh
			client.SubmitWork(job.RawHeader, job.Nonce)
		}
	}()

	go func() {
		for {
			<-timeCh.C
			job := client.GetWork()
			task.ExitJob()
			currentJob = job
			task.Mining(*job)
		}
	}()

	return nil
}

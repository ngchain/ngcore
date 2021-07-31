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

	foundCh := make(chan Job, 1)

	threadNum := 1 // TODO

	currentJob := client.GetWork()
	timeCh := time.NewTicker(time.Second * 2)
	allExitCh := make(chan struct{})
	task := NewMiner(threadNum, foundCh, allExitCh)

	go func() {
		for {
			select {
			case job := <-foundCh:
				client.SubmitWork(job.RawHeader, job.Nonce)
				task.AllExitCh <- struct{}{}
			}
		}
	}()

	go func() {
		for {
			<-timeCh.C
			job := client.GetWork()
			if eq, _ := currentJob.Equals(job.Block); !eq {
				task.AllExitCh <- struct{}{}
				currentJob = job
				task.Mining(*job)
			}
		}
	}()

	return nil
}

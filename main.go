package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/light"
	"github.com/ethereum/go-ethereum/log"
	"github.com/threefoldtech/eth-bridge/api/bridge"
)

func main() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stdout, log.TerminalFormat(true))))

	ctx := context.Background()
	_ = light.LightChain{}
	client, err := ethclient.Dial("https://data-seed-prebsc-1-s1.binance.org:8545")
	if err != nil {
		panic(err)
	}

	t, cancel := context.WithTimeout(ctx, time.Second*15)
	defer cancel()

	id, err := client.ChainID(t)
	if err != nil {
		panic(err)
	}

	log.Debug("Chain ID %+v", id)

	cnl := make(chan struct{})

	br, err := bridge.NewBridge(6969, "/home/dylan/.ethereum/keystore/UTC--2021-04-01T14-12-23.149949737Z--bd330a6f55518b5dc6b984c01dd7f023775fbe7d", "test123", "smart-chain-testnet", nil, "", "./storage", cnl)
	if err != nil {
		panic(err)
	}

	err = br.Start(cnl)
	if err != nil {
		panic(err)
	}

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Debug("signal %+v", sig)
		done <- true
	}()

	log.Debug("awaiting signal")
	<-done
	err = br.Close()
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second * 5)
	log.Debug("exiting")
}

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/light"
	"github.com/threefoldtech/eth-bridge/api/bridge"
)

func main() {
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

	fmt.Println(id)

	cnl := make(chan struct{})

	br, err := bridge.NewBridge(6969, "/home/dylan/.ethereum/keystore/UTC--2021-03-31T15-45-24.047933757Z--ce197b6a9b1f0584be6d33a78eea6eba1e385562", "test123", "smart-chain-testnet", nil, "", "./storage", cnl)
	if err != nil {
		panic(err)
	}

	err = br.Start(cnl)
	if err != nil {
		panic(err)
	}

	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, syscall.SIGINT, syscall.SIGTERM)

	<-exitChan
	cnl <- struct{}{}

	err = br.Close()
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second * 5)
}

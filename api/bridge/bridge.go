package bridge

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"path/filepath"
	"sync"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/stellar/go/amount"
	hProtocol "github.com/stellar/go/protocols/horizon"
	horizoneffects "github.com/stellar/go/protocols/horizon/effects"
)

const (
	// EthBlockDelay is the amount of blocks to wait before
	// pushing eth transaction to the tfchain network
	EthBlockDelay = 30
)

// Bridge is a high lvl structure which listens on contract events and bridge-related
// tfchain transactions, and handles them
type Bridge struct {
	bridgeContract *BridgeContract
	mut            sync.Mutex
}

// NewBridge creates a new Bridge.
func NewBridge(ethPort uint16, accountJSON, accountPass string, ethNetworkName string, bootnodes []string, contractAddress string, datadir string, cancel <-chan struct{}, stellarNetwork string, stellarSeed string) (*Bridge, error) {
	contract, err := NewBridgeContract(ethNetworkName, bootnodes, contractAddress, int(ethPort), accountJSON, accountPass, filepath.Join(datadir, "eth"), cancel, stellarNetwork, stellarSeed)
	if err != nil {
		return nil, err
	}

	bridge := &Bridge{
		bridgeContract: contract,
	}

	return bridge, nil
}

// Close bridge
func (bridge *Bridge) Close() error {
	bridge.mut.Lock()
	defer bridge.mut.Unlock()
	err := bridge.bridgeContract.Close()
	return err
}

func (bridge *Bridge) mint(receiver ERC20Address, amount *big.Int, txID string) error {
	log.Info(fmt.Sprintf("Minting transaction for %s", hex.EncodeToString(receiver[:])))
	// check if we already know this ID
	known, err := bridge.bridgeContract.IsMintTxID(txID)
	if err != nil {
		return err
	}
	if known {
		// we already know this withdrawal address, so ignore the transaction
		return nil
	}
	return bridge.bridgeContract.Mint(receiver, amount, txID)
}

// GetClient returns bridgecontract lightclient
func (bridge *Bridge) GetClient() *LightClient {
	return bridge.bridgeContract.LightClient()
}

// GetBridgeContract returns this bridge's contract.
func (bridge *Bridge) GetBridgeContract() *BridgeContract {
	return bridge.bridgeContract
}

// Start the main processing loop of the bridge
func (bridge *Bridge) Start(cancel <-chan struct{}) error {
	heads := make(chan *ethtypes.Header)

	go bridge.bridgeContract.Loop(heads)

	// subscribing to these events is not needed for operational purposes, but might be nice to get some info
	go bridge.bridgeContract.SubscribeTransfers()
	go bridge.bridgeContract.SubscribeMint()

	withdrawChan := make(chan WithdrawEvent)
	go bridge.bridgeContract.SubscribeWithdraw(withdrawChan, 0)

	go func() {
		txMap := make(map[string]WithdrawEvent)
		for {
			select {
			// // Remember new withdraws
			case we := <-withdrawChan:
				log.Info("Remembering withdraw event", "txHash", we.TxHash(), "height", we.BlockHeight())
				txMap[we.txHash.String()] = we
			// If we get a new head, check every withdraw we have to see if it has matured
			case head := <-heads:
				bridge.mut.Lock()
				for id := range txMap {
					we := txMap[id]
					if head.Number.Uint64() >= we.blockHeight+1 {
						log.Info("Attempting to create an ERC20 withdraw tx", "ethTx", we.TxHash())

						err := bridge.bridgeContract.wallet.CreateAndSubmitPayment(we.blockchain_address, we.amount.Uint64())
						if err != nil {
							log.Error(fmt.Sprintf("failed to create payment for withdrawal to %s", we.blockchain_address), err.Error())
							continue
						}
						// forget about our tx
						delete(txMap, id)
					}
				}

				// bridge.persist.EthHeight = head.Number.Uint64() - EthBlockDelay
				// // Check for underflow
				// if bridge.persist.EthHeight > head.Number.Uint64() {
				// 	bridge.persist.EthHeight = 0
				// }
				// if err := bridge.save(); err != nil {
				// 	log.Error("Failed to save bridge persistency", "err", err)
				// }

				bridge.mut.Unlock()
			}
		}
	}()

	cancelCtx, cnl := context.WithCancel(context.Background())
	go func() {
		time.Sleep(5 * time.Minute)
		cnl()
	}()

	transactionHandler := func(tx hProtocol.Transaction) {
		if !tx.Successful {
			return
		}

		data, err := base64.StdEncoding.DecodeString(tx.Memo)
		if err != nil {
			fmt.Println("error:", err)
			return
		}

		if len(data) != 20 {
			return
		}
		var ethAddress ERC20Address
		copy(ethAddress[0:20], data)

		fmt.Println(hex.EncodeToString(ethAddress[:]))

		effects, err := bridge.bridgeContract.GetTransactionEffects(tx.Hash)
		if err != nil {
			return
		}

		for _, effect := range effects.Embedded.Records {
			if effect.GetAccount() != "GBC3XAFKPN6RDL4MEDZCYS3GOP3ANRCHRY5722UUJ52RVVODVMMWAGTJ" {
				continue
			}
			if effect.GetType() == "account_credited" {
				creditedEffect := effect.(horizoneffects.AccountCredited)
				if creditedEffect.Asset.Code != "TFT" {
					continue
				}
				parsedAmount, err := amount.ParseInt64(creditedEffect.Amount)
				if err != nil {
					continue
				}

				eth_amount := big.NewInt(int64(parsedAmount))

				err = bridge.mint(ethAddress, eth_amount, tx.Hash)
				if err != nil {
					fmt.Println("error while minting")
					fmt.Println(err)
					return
				}
				log.Info("Mint succesfull")
			}
		}

	}

	go bridge.bridgeContract.StreamStellarAccountTransactions(cancelCtx, "GBC3XAFKPN6RDL4MEDZCYS3GOP3ANRCHRY5722UUJ52RVVODVMMWAGTJ", transactionHandler)

	return nil
}

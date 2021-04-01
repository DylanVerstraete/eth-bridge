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
	// TFTBlockDelay is the amount of blocks to wait before
	// pushing tft transactions to the ethereum contract
	TFTBlockDelay = 6
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
func NewBridge(ethPort uint16, accountJSON, accountPass string, ethNetworkName string, bootnodes []string, contractAddress string, datadir string, cancel <-chan struct{}) (*Bridge, error) {
	contract, err := NewBridgeContract(ethNetworkName, bootnodes, contractAddress, int(ethPort), accountJSON, accountPass, filepath.Join(datadir, "eth"), cancel)
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
	go bridge.bridgeContract.SubscribeRegisterWithdrawAddress()

	// withdrawChan := make(chan WithdrawEvent)

	// TODO
	// go bridge.bridgeContract.SubscribeWithdraw(withdrawChan, 0)
	go func() {
		// txMap := make(map[erc20types.ERC20Hash]WithdrawEvent)
		for {
			select {
			// // Remember new withdraws
			// case we := <-withdrawChan:
			// 	// Check if the withdraw is valid
			// 	_, found, err := erc20Registry.GetTFTAddressForERC20Address(erc20types.ERC20Address(we.receiver))
			// 	if err != nil {
			// 		log.Error(fmt.Sprintf("Retrieving TFT address for registered ERC20 address %v errored: %v", we.receiver, err))
			// 		continue
			// 	}
			// 	if !found {
			// 		log.Error(fmt.Sprintf("Failed to retrieve TFT address for registered ERC20 Withdrawal address %v", we.receiver))
			// 		continue
			// 	}
			// 	// remember the withdraw
			// 	txMap[erc20types.ERC20Hash(we.txHash)] = we
			// 	fmt.Println("Remembering withdraw event", "txHash", we.TxHash(), "height", we.BlockHeight())

			// If we get a new head, check every withdraw we have to see if it has matured
			case _ = <-heads:
				// fmt.Println(head.Number)

				// cl := bridge.GetClient()
				// fmt.Println(cl.lesc.Downloader().Progress())
				// p, _ := cl.PendingTransactionCount(context.Background())

				// fmt.Println(p)
				// p, err := bridge.bridgeContract.lc.SyncProgress(context.Background())
				// if err != nil {
				// 	fmt.Println(err)
				// }
				// fmt.Println(p)

				// b, err := bridge.bridgeContract.lc.BalanceAt(context.Background(), common.HexToAddress("0xbD330A6F55518b5dc6B984c01dd7f023775fbe7d"), head.Number)
				// if err != nil {
				// 	fmt.Println(err)
				// }

				// fmt.Println(b)
				// bridge.mut.Lock()
				// for id := range txMap {
				// 	we := txMap[id]
				// 	if head.Number.Uint64() >= we.blockHeight+EthBlockDelay {
				// 		fmt.Println("Attempting to create an ERC20 withdraw tx", "ethTx", we.TxHash())
				// 		// we waited long enough, create transaction and push it
				// 		uh, found, err := erc20Registry.GetTFTAddressForERC20Address(erc20types.ERC20Address(we.receiver))
				// 		if err != nil {
				// 			log.Error(fmt.Sprintf("Retrieving TFT address for registered ERC20 address %v errored: %v", we.receiver, err))
				// 			continue
				// 		}
				// 		if !found {
				// 			log.Error(fmt.Sprintf("Failed to retrieve TFT address for registered ERC20 Withdrawal address %v", we.receiver))
				// 			continue
				// 		}

				// 		tx := erc20types.ERC20CoinCreationTransaction{}
				// 		tx.Address = uh

				// 		// define the txFee
				// 		tx.TransactionFee = bridge.chainCts.MinimumTransactionFee

				// 		// define the value, which is the value withdrawn minus the fees
				// 		tx.Value = types.NewCurrency(we.amount).Sub(tx.TransactionFee)

				// 		// fill in the other info
				// 		tx.TransactionID = erc20types.ERC20Hash(we.txHash)
				// 		tx.BlockID = erc20types.ERC20Hash(we.blockHash)

				// 		if err := bridge.commitWithdrawTransaction(tx); err != nil {
				// 			log.Error("Failed to create ERC20 Withdraw transaction", "err", err)
				// 			continue
				// 		}

				// 		fmt.Println("Created ERC20 -> TFT transaction", "txid", tx.Transaction(bridge.txVersions.ERC20CoinCreation).ID())

				// 		// forget about our tx
				// 		delete(txMap, id)
				// 	}
				// }

				// bridge.persist.EthHeight = head.Number.Uint64() - EthBlockDelay
				// // Check for underflow
				// if bridge.persist.EthHeight > head.Number.Uint64() {
				// 	bridge.persist.EthHeight = 0
				// }
				// if err := bridge.save(); err != nil {
				// 	log.Error("Failed to save bridge persistency", "err", err)
				// }

				// bridge.mut.Unlock()
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

package main

import (
	"github.com/boltdb/bolt"
	"os"
	"fmt"
	"encoding/hex"
	"log"
)

const (
	dbFile       = "blockchain.db"
	blocksBucket = "blocks"
	genesisCoinbaseData = "05/Jan/2018 KublaiCoin is now a thing."
)

type BlockChain struct {
	tip []byte
	db  *bolt.DB
}

type BlockChainIterator struct {
	currentHash []byte
	db          *bolt.DB
}

func (blockChain *BlockChain) Iterator() *BlockChainIterator {
	iterator := &BlockChainIterator{blockChain.tip, blockChain.db}
	return iterator
}

func (i *BlockChainIterator) Next() *Block {
	var block *Block

	i.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		encodedBlock := b.Get(i.currentHash)
		block = DeserializeBlock(encodedBlock)

		return nil
	})

	i.currentHash = block.PrevBlockHash

	return block
}

func (blockChain *BlockChain) AddBlock(transactions []*Transaction) {
	var lastHash []byte

	err := blockChain.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		lastHash = b.Get([]byte("l"))

		return nil
	})

	newBlock := NewBlock(transactions, lastHash)

	blockChain.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		err = b.Put(newBlock.Hash, newBlock.Serialize())
		err = b.Put([]byte("l"), newBlock.Hash)
		blockChain.tip = newBlock.Hash

		return nil
	})
}

func (blockChain *BlockChain) FindUnspentTransactions(address string) []Transaction {
	var unspentTransactions []Transaction

	spentTransactions := make(map[string][]int)
	iterator := blockChain.Iterator()

	for {
		block := iterator.Next()

		for _, transaction := range block.Transactions {
			transactionId := hex.EncodeToString(transaction.ID)

			Outputs:
				for outIdx, out := range transaction.Vout {
					// Check if the output was spent
					if spentTransactions[transactionId] != nil {
						for _, spentOut := range spentTransactions[transactionId] {
							if spentOut == outIdx {
								continue Outputs
							}
						}
					}

					if out.CanBeUnlockedWith(address) {
						unspentTransactions = append(unspentTransactions, *transaction)
					}
				}

			if transaction.IsCoinbase() == false {
				for _, in := range transaction.Vin {
					if in.CanUnlockOutputWith(address) {
						inTxID := hex.EncodeToString(in.TXId)
						spentTransactions[inTxID] = append(spentTransactions[inTxID], in.Vout)
					}
				}
			}
		}

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return unspentTransactions
}

func (bc *BlockChain) FindUTXO(address string) []TXOutput {
	var UTXOs []TXOutput
	unspentTransactions := bc.FindUnspentTransactions(address)

	for _, tx := range unspentTransactions {
		for _, out := range tx.Vout {
			if out.CanBeUnlockedWith(address) {
				UTXOs = append(UTXOs, out)
			}
		}
	}

	return UTXOs
}

// NewBlockChain creates a new database, blockchain, and genesis block
func NewBlockChain(address string) *BlockChain {
	var tip []byte

	if dbExists() {
		fmt.Println("Kublacoin blockchain already exists. Aborting...")
		os.Exit(1)
	}

	db, err := bolt.Open(dbFile, 0600, nil)

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		if b == nil {
			coinbaseTransaction := NewCoinbaseTx(address, genesisCoinbaseData)
			genesis := NewGenesisBlock(coinbaseTransaction)
			b, _ := tx.CreateBucket([]byte(blocksBucket))
			err = b.Put(genesis.Hash, genesis.Serialize())
			err = b.Put([]byte("l"), genesis.Hash)
			tip = genesis.Hash
		} else {
			tip = b.Get([]byte("l"))
		}

		return nil
	})

	bc := BlockChain{tip, db}

	return &bc
}

// GetBlockchain gets an existing blockchain
func GetBlockchain() *BlockChain {
	if !dbExists() {
		fmt.Println("No existing blockchain found. Create one first.")
		os.Exit(1)
	}

	var tip []byte
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		tip = b.Get([]byte("l"))

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	bc := BlockChain{tip, db}

	return &bc
}

func dbExists() bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}

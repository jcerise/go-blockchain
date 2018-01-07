package main

import (
	"encoding/hex"
	"fmt"
	"github.com/boltdb/bolt"
	"log"
	"os"
)

const (
	dbFile              = "blockchain.db"
	blocksBucket        = "blocks"
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

func (blockchain *BlockChain) MineBlock(transactions []*Transaction) {
	var lastHash []byte

	err := blockchain.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		lastHash = b.Get([]byte("l"))

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	newBlock := NewBlock(transactions, lastHash)

	err = blockchain.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		err := b.Put(newBlock.Hash, newBlock.Serialize())
		if err != nil {
			log.Panic(err)
		}

		err = b.Put([]byte("l"), newBlock.Hash)
		if err != nil {
			log.Panic(err)
		}

		blockchain.tip = newBlock.Hash

		return nil
	})
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

func NewUTXOTransaction(from, to string, amount int, blockchain *BlockChain) *Transaction {
	var inputs []TXInput
	var outputs []TXOutput

	acc, validOutputs := blockchain.FindSpendableOutputs(from, amount)

	if acc < amount {
		log.Panic("ERROR: Not enough funds")
	}

	// Build a list of inputs
	for txid, outs := range validOutputs {
		txID, _ := hex.DecodeString(txid)

		for _, out := range outs {
			input := TXInput{txID, out, from}
			inputs = append(inputs, input)
		}
	}

	// Build a list of outputs
	outputs = append(outputs, TXOutput{amount, to})
	if acc > amount {
		outputs = append(outputs, TXOutput{acc - amount, from}) // a change
	}

	tx := Transaction{nil, inputs, outputs}
	tx.SetId()

	return &tx
}

func (blockchain *BlockChain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	unspentOutputs := make(map[string][]int)
	unspentTXs := blockchain.FindUnspentTransactions(address)
	accumulated := 0

Work:
	for _, tx := range unspentTXs {
		txID := hex.EncodeToString(tx.ID)

		for outIdx, out := range tx.Vout {
			if out.CanBeUnlockedWith(address) && accumulated < amount {
				accumulated += out.Value
				unspentOutputs[txID] = append(unspentOutputs[txID], outIdx)

				if accumulated >= amount {
					break Work
				}
			}
		}
	}

	return accumulated, unspentOutputs
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

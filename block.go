package main

import (
	"bytes"
	"encoding/gob"
	"time"
	"crypto/sha256"
)

type Block struct {
	Timestamp     int64
	Transactions []*Transaction
	PrevBlockHash []byte
	Hash          []byte
	Nonce         int
}

func NewGenesisBlock(coinbase *Transaction) *Block {
	return NewBlock([]*Transaction{coinbase}, []byte{})
}

func NewBlock(transactions []*Transaction, prevBlockHash []byte) *Block {
	block := &Block{time.Now().Unix(), transactions, prevBlockHash, []byte{}, 0}
	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()

	block.Hash = hash[:]
	block.Nonce = nonce

	return block
}

func (block *Block) Serialize() []byte {
	var results bytes.Buffer

	encoder := gob.NewEncoder(&results)

	// This is lazy and needs some error handling
	encoder.Encode(block)

	return results.Bytes()
}

func (block *Block) HashTransactions() []byte  {
	var transactionHashes [][]byte
	var transactionHash [32]byte

	for _, tx := range block.Transactions {
		transactionHashes = append(transactionHashes, tx.ID)
	}

	transactionHash = sha256.Sum256(bytes.Join(transactionHashes, []byte{}))

	return transactionHash[:]
}

func DeserializeBlock(d []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(d))
	decoder.Decode(&block)

	return &block
}

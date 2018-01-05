package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
)

type CLI struct {}

func (cli *CLI) printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  initialize -address ADDRESS - Create a new blockchain, which will send the reward from the genesis block to ADDRESS")
	fmt.Println("  balance - address ADDRESS - Check the unspent balance of ADDRESS")
	fmt.Println("  print - prints all the blocks present in the blockchain in reverse order")
}

func (cli *CLI) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		os.Exit(1)
	}
}

func (cli *CLI) Run() {
	cli.validateArgs()

	initializeCmd := flag.NewFlagSet("initialize", flag.ExitOnError)
	balanceCmd := flag.NewFlagSet("balance", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("print", flag.ExitOnError)

	initializeData := initializeCmd.String("address", "", "Genesis Block reward address")
	balanceData := balanceCmd.String("address", "", "Address to check balance of.")


	switch os.Args[1] {
	case "initialize":
		initializeCmd.Parse(os.Args[2:])
	case "balance":
		balanceCmd.Parse(os.Args[2:])
	case "print":
		printChainCmd.Parse(os.Args[2:])
	default:
		cli.printUsage()
		os.Exit(1)
	}

	if initializeCmd.Parsed() {
		if *initializeData == "" {
			initializeCmd.Usage()
			os.Exit(1)
		}
		cli.createBlockChain(*initializeData)
	}

	if balanceCmd.Parsed() {
		if *balanceData == "" {
			balanceCmd.Usage()
			os.Exit(1)
		}
		cli.getBalance(*balanceData)
	}

	if printChainCmd.Parsed() {
		cli.printChain()
	}
}

func (cli *CLI) createBlockChain(address string) {
	blockChain := NewBlockChain(address)

	blockChain.db.Close()

	fmt.Println("New KublaiCoin blockchain created.")
}

func (cli *CLI) getBalance(address string) {
	blockChain := GetBlockchain()

	defer blockChain.db.Close()

	balance := 0
	UTXOs := blockChain.FindUTXO(address)

	for _, out := range UTXOs {
		balance += out.Value
	}

	fmt.Printf("Balance of '%s': %d\n", address, balance)
}

func (cli *CLI) printChain() {
	blockChain := NewBlockChain("")
	defer blockChain.db.Close()

	bci := blockChain.Iterator()

	for {
		block := bci.Next()

		fmt.Printf("Prev. hash: %x\n", block.PrevBlockHash)
		fmt.Printf("Hash: %x\n", block.Hash)
		fmt.Printf("PoW Nonce: %v\n", block.Nonce)
		pow := NewProofOfWork(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()

		if len(block.PrevBlockHash) == 0 {
			break
		}
	}
}

package main


type BlockChain struct {
	Blocks []*Block
}

func (blockChain *BlockChain) AddBlock(data string) {
	prevBlock := blockChain.Blocks[len(blockChain.Blocks) - 1]
	newBlock := NewBlock(data, prevBlock.Hash)
	blockChain.Blocks = append(blockChain.Blocks, newBlock)
}

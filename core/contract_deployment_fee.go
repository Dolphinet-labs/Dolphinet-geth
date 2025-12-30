package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
)

const (
	DefaultFeePercentage = 100
	FeeReceiverAddress   = "0x0000000000000000000000000000000000000000"
)

var (
	ContractDeploymentFeeReceiver = common.HexToAddress(FeeReceiverAddress)
)

type ContractDeploymentFeeCalculator struct {
	feePercentage          *big.Int
	totalSupplyStorageAddr common.Address
	totalSupplyStorageSlot common.Hash
}

func NewContractDeploymentFeeCalculator(feePercentage *big.Int, totalSupplyStorageAddr common.Address, totalSupplyStorageSlot common.Hash) *ContractDeploymentFeeCalculator {
	if feePercentage == nil {
		feePercentage = big.NewInt(DefaultFeePercentage)
	}
	return &ContractDeploymentFeeCalculator{
		feePercentage:          feePercentage,
		totalSupplyStorageAddr: totalSupplyStorageAddr,
		totalSupplyStorageSlot: totalSupplyStorageSlot,
	}
}

func (c *ContractDeploymentFeeCalculator) CalculateFee(totalSupply *big.Int) *big.Int {
	if totalSupply == nil || totalSupply.Sign() <= 0 {
		log.Warn("Invalid total supply, returning zero fee", "supply", totalSupply)
		return big.NewInt(0)
	}

	fee := new(big.Int).Mul(totalSupply, c.feePercentage)
	fee.Div(fee, big.NewInt(10000))

	// 确保至少是 1 wei
	if fee.Sign() == 0 {
		fee.SetInt64(1)
	}

	return fee
}

func (c *ContractDeploymentFeeCalculator) GetTotalSupply(evm *vm.EVM) *big.Int {
	if c.totalSupplyStorageAddr == (common.Address{}) {
		log.Warn("Total supply storage address not configured, returning zero")
		return big.NewInt(0)
	}

	storageValue := evm.StateDB.GetState(c.totalSupplyStorageAddr, c.totalSupplyStorageSlot)

	supply := new(big.Int).SetBytes(storageValue[:])

	if supply.Sign() <= 0 {
		log.Warn("Total supply is zero or negative",
			"address", c.totalSupplyStorageAddr.Hex(),
			"slot", c.totalSupplyStorageSlot.Hex(),
			"value", supply)
		return big.NewInt(0)
	}

	log.Debug("Retrieved native token total supply from storage",
		"supply", supply,
		"address", c.totalSupplyStorageAddr.Hex(),
		"slot", c.totalSupplyStorageSlot.Hex())

	return supply
}

func (c *ContractDeploymentFeeCalculator) GetFeeReceiver() common.Address {
	return ContractDeploymentFeeReceiver
}

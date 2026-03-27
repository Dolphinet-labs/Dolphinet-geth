package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

const (
	DefaultFeePercentage = 100
	FeeReceiverAddress   = "" //TODO: change receiver
)

var (
	ContractDeploymentFeeReceiver = common.HexToAddress(FeeReceiverAddress)
)

type ContractDeploymentFeeCalculator struct {
	feePercentage *big.Int
}

func NewContractDeploymentFeeCalculator(feePercentage *big.Int) *ContractDeploymentFeeCalculator {
	if feePercentage == nil {
		feePercentage = big.NewInt(DefaultFeePercentage)
	}
	return &ContractDeploymentFeeCalculator{
		feePercentage: feePercentage,
	}
}

func (c *ContractDeploymentFeeCalculator) CalculateFee(totalSupply *big.Int) *big.Int {
	if totalSupply == nil || totalSupply.Sign() <= 0 {
		log.Warn("Invalid total supply, returning zero fee", "supply", totalSupply)
		return big.NewInt(0)
	}

	fee := new(big.Int).Mul(totalSupply, c.feePercentage)
	fee.Div(fee, big.NewInt(10000))

	if fee.Sign() == 0 {
		fee.SetInt64(1)
	}

	return fee
}

func (c *ContractDeploymentFeeCalculator) GetTotalSupply() *big.Int {
	totalSupply, _ := new(big.Int).SetString("2100000000000000000000000000", 10)
	return totalSupply
}

func (c *ContractDeploymentFeeCalculator) GetFeeReceiver() common.Address {
	return ContractDeploymentFeeReceiver
}

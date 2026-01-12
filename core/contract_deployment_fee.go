package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
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

	if fee.Sign() == 0 {
		fee.SetInt64(1)
	}

	return fee
}

func (c *ContractDeploymentFeeCalculator) GetTotalSupply(evm *vm.EVM) *big.Int {
	// First, try to get total supply from supply tracer via the registered function
	if getTotalSupplyFunc != nil {
		if totalSupply := getTotalSupplyFunc(); totalSupply != nil && totalSupply.Sign() > 0 {
			log.Debug("Retrieved native token total supply from supply tracer",
				"supply", totalSupply)
			return totalSupply
		}
	}

	// Last resort: return default value
	log.Warn("Total supply not available from supply tracer or storage, returning default")
	totalSupply, _ := new(big.Int).SetString("1000000000000000000000000000", 10)
	return totalSupply
}

// TotalSupplyGetter is a function type for getting total supply
// This matches live.TotalSupplyGetterFunc to avoid type conversion
type TotalSupplyGetter func() *big.Int

var (
	getTotalSupplyFunc TotalSupplyGetter
)

// SetTotalSupplyGetter sets the function to get total supply from supply tracer
// This should be called by the supply tracer during initialization
// It accepts any function with signature func() *big.Int
func SetTotalSupplyGetter(getter func() *big.Int) {
	getTotalSupplyFunc = getter
}

func (c *ContractDeploymentFeeCalculator) GetFeeReceiver() common.Address {
	return ContractDeploymentFeeReceiver
}

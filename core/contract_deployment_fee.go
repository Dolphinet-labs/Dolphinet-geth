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
	// First, try to get total supply from supply tracer via the registered function
	log.Info("GetTotalSupply called", "getTotalSupplyFunc is nil", getTotalSupplyFunc == nil)
	if getTotalSupplyFunc != nil {
		log.Info("Calling getTotalSupplyFunc")
		if totalSupply := getTotalSupplyFunc(); totalSupply != nil && totalSupply.Sign() > 0 {
			log.Debug("Retrieved native token total supply from supply tracer",
				"supply", totalSupply)
			return totalSupply
		} else {
			log.Warn("getTotalSupplyFunc returned nil or zero", "supply", totalSupply)
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
	log.Info("SetTotalSupplyGetter called", "getter is nil", getter == nil)
	getTotalSupplyFunc = getter
	log.Info("SetTotalSupplyGetter completed", "getTotalSupplyFunc is nil", getTotalSupplyFunc == nil)
}

func (c *ContractDeploymentFeeCalculator) GetFeeReceiver() common.Address {
	return ContractDeploymentFeeReceiver
}

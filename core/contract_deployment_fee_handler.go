package core

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/holiman/uint256"
)

func chargeContractDeploymentFeeIfNeeded(evm *vm.EVM, from common.Address, statedb *state.StateDB, validatorChecker ValidatorChecker, blockNumber uint64) error {
	var posActive bool
	var posBlock uint64
	if evm.ChainConfig() != nil && evm.ChainConfig().DolphinetPoSBlock != nil {
		posBlock = evm.ChainConfig().DolphinetPoSBlock.Uint64()
		if blockNumber >= posBlock {
			posActive = true
		}
	}
	log.Debug("chargeContractDeploymentFeeIfNeeded entry",
		"from", from.Hex(),
		"blockNumber", blockNumber,
		"posBlock", posBlock,
		"posActive", posActive,
		"validatorCheckerNil", validatorChecker == nil,
	)
	if !posActive {
		return nil
	}
	feeCalculator := NewContractDeploymentFeeCalculator(big.NewInt(100))

	if feeCalculator == nil {
		log.Debug("feeCalculator is nil")
		return nil
	}

	isValidator := false
	if validatorChecker != nil {
		isValidator = validatorChecker.IsValidator(from, evm)
	}
	if isValidator {
		log.Debug("Validator contract deployment, skipping extra fee", "from", from.Hex())
		return nil
	}

	totalSupply := feeCalculator.GetTotalSupply()
	if totalSupply == nil || totalSupply.Sign() <= 0 {
		log.Warn("Failed to get total supply, skipping contract deployment fee", "from", from.Hex())
		return nil
	}

	extraFee := feeCalculator.CalculateFee(totalSupply)
	if extraFee.Sign() <= 0 {
		log.Warn("Calculated fee is zero or negative, skipping", "from", from.Hex(), "fee", extraFee)
		return nil
	}

	feeReceiver := feeCalculator.GetFeeReceiver()

	balance := statedb.GetBalance(from)
	extraFeeU256, overflow := uint256.FromBig(extraFee)
	if overflow {
		return fmt.Errorf("extra fee overflow: %v", extraFee)
	}

	if balance.Cmp(extraFeeU256) < 0 {
		return fmt.Errorf(
			"insufficient funds for contract deployment fee: address %v, balance %v, required fee %v (1%% of total supply %v)",
			from.Hex(), balance, extraFeeU256, totalSupply,
		)
	}

	statedb.SubBalance(from, extraFeeU256, tracing.BalanceDecreaseGasBuy)
	statedb.AddBalance(feeReceiver, extraFeeU256, tracing.BalanceIncreaseRewardTransactionFee)

	log.Info("Charged contract deployment fee",
		"from", from.Hex(),
		"fee", extraFee,
		"totalSupply", totalSupply,
		"feeReceiver", feeReceiver.Hex(),
	)

	return nil
}

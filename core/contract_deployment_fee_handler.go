package core

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/holiman/uint256"
)

func chargeContractDeploymentFeeIfNeeded(evm *vm.EVM, from common.Address, statedb *state.StateDB) error {
	validatorChecker := evm.Config.ValidatorChecker
	feeCalculator := evm.Config.ContractDeploymentFeeCalculator

	if validatorChecker == nil || feeCalculator == nil {
		return nil
	}

	isValidator := validatorChecker.IsValidator(from, evm)
	if isValidator {
		log.Debug("Validator contract deployment, skipping extra fee", "from", from.Hex())
		return nil
	}

	totalSupply := feeCalculator.GetTotalSupply(evm)
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

	statedb.SubBalance(from, extraFeeU256, tracing.BalanceDecreaseGasBuy)                      // 暂时使用 GasBuy，后续可以添加专门的常量
	statedb.AddBalance(feeReceiver, extraFeeU256, tracing.BalanceIncreaseRewardTransactionFee) // 暂时使用 TransactionFee

	log.Info("Charged contract deployment fee",
		"from", from.Hex(),
		"fee", extraFee,
		"totalSupply", totalSupply,
		"feeReceiver", feeReceiver.Hex(),
	)

	return nil
}

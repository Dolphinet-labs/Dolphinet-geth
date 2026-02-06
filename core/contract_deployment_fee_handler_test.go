package core

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

func createTestEVMForHandler(validatorChecker interface {
	IsValidator(common.Address, *vm.EVM) bool
}, feeCalculator interface {
	GetTotalSupply(*vm.EVM) *big.Int
	CalculateFee(*big.Int) *big.Int
	GetFeeReceiver() common.Address
}) (*vm.EVM, *state.StateDB) {
	db := state.NewDatabaseForTesting()
	statedb, _ := state.New(types.EmptyRootHash, db)

	header := &types.Header{
		Number:     big.NewInt(1),
		Time:       1000,
		Difficulty: big.NewInt(0),
		GasLimit:   10000000,
	}

	coinbase := common.Address{0x01}
	blockCtx := NewEVMBlockContext(header, nil, &coinbase, params.TestChainConfig, statedb)
	evmConfig := vm.Config{
		ValidatorChecker:                validatorChecker,
		ContractDeploymentFeeCalculator: feeCalculator,
	}
	evm := vm.NewEVM(blockCtx, statedb, params.TestChainConfig, evmConfig)

	return evm, statedb
}

// mockValidatorChecker is a simple mock implementation for testing
type mockValidatorChecker struct {
	validators map[common.Address]bool
}

func newMockValidatorChecker(validators []common.Address) *mockValidatorChecker {
	m := &mockValidatorChecker{
		validators: make(map[common.Address]bool),
	}
	for _, addr := range validators {
		m.validators[addr] = true
	}
	return m
}

func (m *mockValidatorChecker) IsValidator(addr common.Address, evm *vm.EVM) bool {
	return m.validators[addr]
}

// mockFeeCalculator is a simple mock implementation for testing
type mockFeeCalculator struct {
	totalSupply *big.Int
	feeReceiver common.Address
}

func newMockFeeCalculator(totalSupply *big.Int, feeReceiver common.Address) *mockFeeCalculator {
	return &mockFeeCalculator{
		totalSupply: totalSupply,
		feeReceiver: feeReceiver,
	}
}

func (m *mockFeeCalculator) CalculateFee(totalSupply *big.Int) *big.Int {
	if totalSupply == nil || totalSupply.Sign() <= 0 {
		return big.NewInt(0)
	}
	fee := new(big.Int).Mul(totalSupply, big.NewInt(100))
	fee.Div(fee, big.NewInt(10000))
	// Return 0 if fee is 0, don't force it to 1
	return fee
}

func (m *mockFeeCalculator) GetTotalSupply(evm *vm.EVM) *big.Int {
	return m.totalSupply
}

func (m *mockFeeCalculator) GetFeeReceiver() common.Address {
	return m.feeReceiver
}

func TestChargeContractDeploymentFeeIfNeeded_ValidatorExempt(t *testing.T) {
	validatorAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")
	validatorChecker := newMockValidatorChecker([]common.Address{validatorAddr})
	totalSupply, _ := new(big.Int).SetString("1000000000000000000000000000", 10)
	feeCalculator := newMockFeeCalculator(totalSupply, common.HexToAddress("0x9999999999999999999999999999999999999999"))

	evm, statedb := createTestEVMForHandler(validatorChecker, feeCalculator)

	// Set balance for validator
	statedb.AddBalance(validatorAddr, uint256.NewInt(1000000000000000000), tracing.BalanceChangeUnspecified)

	initialBalance := statedb.GetBalance(validatorAddr)
	feeReceiverBalance := statedb.GetBalance(feeCalculator.GetFeeReceiver())

	err := chargeContractDeploymentFeeIfNeeded(evm, validatorAddr, statedb, nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Validator balance should not change
	finalBalance := statedb.GetBalance(validatorAddr)
	if finalBalance.Cmp(initialBalance) != 0 {
		t.Errorf("Validator balance should not change, initial: %v, final: %v", initialBalance, finalBalance)
	}

	// Fee receiver balance should not change
	finalFeeReceiverBalance := statedb.GetBalance(feeCalculator.GetFeeReceiver())
	if finalFeeReceiverBalance.Cmp(feeReceiverBalance) != 0 {
		t.Errorf("Fee receiver balance should not change, initial: %v, final: %v", feeReceiverBalance, finalFeeReceiverBalance)
	}
}

func TestChargeContractDeploymentFeeIfNeeded_NonValidatorCharged(t *testing.T) {
	validatorAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")
	nonValidatorAddr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	feeReceiverAddr := common.HexToAddress("0x9999999999999999999999999999999999999999")

	validatorChecker := newMockValidatorChecker([]common.Address{validatorAddr})
	totalSupply, _ := new(big.Int).SetString("1000000000000000000000000000", 10) // 1e27
	feeCalculator := newMockFeeCalculator(totalSupply, feeReceiverAddr)

	evm, statedb := createTestEVMForHandler(validatorChecker, feeCalculator)

	// Set balance for non-validator (enough to pay fee)
	expectedFee := feeCalculator.CalculateFee(totalSupply)
	sufficientBalance := new(big.Int).Mul(expectedFee, big.NewInt(2)) // 2x the fee
	statedb.AddBalance(nonValidatorAddr, uint256.MustFromBig(sufficientBalance), tracing.BalanceChangeUnspecified)

	initialFeeReceiverBalance := statedb.GetBalance(feeReceiverAddr)

	err := chargeContractDeploymentFeeIfNeeded(evm, nonValidatorAddr, statedb, nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Non-validator balance should decrease by fee
	finalBalance := statedb.GetBalance(nonValidatorAddr)
	expectedFinalBalance := new(big.Int).Sub(sufficientBalance, expectedFee)
	if finalBalance.Cmp(uint256.MustFromBig(expectedFinalBalance)) != 0 {
		t.Errorf("Non-validator balance incorrect, expected: %v, got: %v", expectedFinalBalance, finalBalance)
	}

	// Fee receiver balance should increase by fee
	finalFeeReceiverBalance := statedb.GetBalance(feeReceiverAddr)
	expectedFeeReceiverBalance := new(big.Int).Add(initialFeeReceiverBalance.ToBig(), expectedFee)
	if finalFeeReceiverBalance.Cmp(uint256.MustFromBig(expectedFeeReceiverBalance)) != 0 {
		t.Errorf("Fee receiver balance incorrect, expected: %v, got: %v", expectedFeeReceiverBalance, finalFeeReceiverBalance)
	}
}

func TestChargeContractDeploymentFeeIfNeeded_InsufficientBalance(t *testing.T) {
	nonValidatorAddr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	feeReceiverAddr := common.HexToAddress("0x9999999999999999999999999999999999999999")

	validatorChecker := newMockValidatorChecker([]common.Address{})
	totalSupply, _ := new(big.Int).SetString("1000000000000000000000000000", 10)
	feeCalculator := newMockFeeCalculator(totalSupply, feeReceiverAddr)

	evm, statedb := createTestEVMForHandler(validatorChecker, feeCalculator)

	// Set insufficient balance
	expectedFee := feeCalculator.CalculateFee(totalSupply)
	insufficientBalance := new(big.Int).Div(expectedFee, big.NewInt(2)) // Half of the fee
	statedb.AddBalance(nonValidatorAddr, uint256.MustFromBig(insufficientBalance), tracing.BalanceChangeUnspecified)

	err := chargeContractDeploymentFeeIfNeeded(evm, nonValidatorAddr, statedb, nil)

	if err == nil {
		t.Error("Expected error for insufficient balance, got nil")
	}

	// Balance should not change on error
	finalBalance := statedb.GetBalance(nonValidatorAddr)
	if finalBalance.Cmp(uint256.MustFromBig(insufficientBalance)) != 0 {
		t.Errorf("Balance should not change on error, expected: %v, got: %v", insufficientBalance, finalBalance)
	}
}

func TestChargeContractDeploymentFeeIfNeeded_NilValidatorChecker(t *testing.T) {
	nonValidatorAddr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	totalSupply, _ := new(big.Int).SetString("1000000000000000000000000000", 10)
	feeCalculator := newMockFeeCalculator(totalSupply, common.HexToAddress("0x9999999999999999999999999999999999999999"))

	evm, statedb := createTestEVMForHandler(nil, feeCalculator)

	statedb.AddBalance(nonValidatorAddr, uint256.NewInt(1000000000000000000), tracing.BalanceChangeUnspecified)

	initialBalance := statedb.GetBalance(nonValidatorAddr)

	err := chargeContractDeploymentFeeIfNeeded(evm, nonValidatorAddr, statedb, nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Balance should not change when validator checker is nil
	finalBalance := statedb.GetBalance(nonValidatorAddr)
	if finalBalance.Cmp(initialBalance) != 0 {
		t.Errorf("Balance should not change when validator checker is nil, initial: %v, final: %v", initialBalance, finalBalance)
	}
}

func TestChargeContractDeploymentFeeIfNeeded_NilFeeCalculator(t *testing.T) {
	nonValidatorAddr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	validatorChecker := newMockValidatorChecker([]common.Address{})

	evm, statedb := createTestEVMForHandler(validatorChecker, nil)

	statedb.AddBalance(nonValidatorAddr, uint256.NewInt(1000000000000000000), tracing.BalanceChangeUnspecified)

	initialBalance := statedb.GetBalance(nonValidatorAddr)

	err := chargeContractDeploymentFeeIfNeeded(evm, nonValidatorAddr, statedb, nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Balance should not change when fee calculator is nil
	finalBalance := statedb.GetBalance(nonValidatorAddr)
	if finalBalance.Cmp(initialBalance) != 0 {
		t.Errorf("Balance should not change when fee calculator is nil, initial: %v, final: %v", initialBalance, finalBalance)
	}
}

func TestChargeContractDeploymentFeeIfNeeded_ZeroTotalSupply(t *testing.T) {
	nonValidatorAddr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	feeReceiverAddr := common.HexToAddress("0x9999999999999999999999999999999999999999")

	validatorChecker := newMockValidatorChecker([]common.Address{})
	feeCalculator := newMockFeeCalculator(big.NewInt(0), feeReceiverAddr) // Zero supply

	evm, statedb := createTestEVMForHandler(validatorChecker, feeCalculator)

	statedb.AddBalance(nonValidatorAddr, uint256.NewInt(1000000000000000000), tracing.BalanceChangeUnspecified)

	initialBalance := statedb.GetBalance(nonValidatorAddr)

	err := chargeContractDeploymentFeeIfNeeded(evm, nonValidatorAddr, statedb, nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Balance should not change when total supply is zero
	finalBalance := statedb.GetBalance(nonValidatorAddr)
	if finalBalance.Cmp(initialBalance) != 0 {
		t.Errorf("Balance should not change when total supply is zero, initial: %v, final: %v", initialBalance, finalBalance)
	}
}

func TestChargeContractDeploymentFeeIfNeeded_NegativeTotalSupply(t *testing.T) {
	nonValidatorAddr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	feeReceiverAddr := common.HexToAddress("0x9999999999999999999999999999999999999999")

	validatorChecker := newMockValidatorChecker([]common.Address{})
	feeCalculator := newMockFeeCalculator(big.NewInt(-1000), feeReceiverAddr) // Negative supply

	evm, statedb := createTestEVMForHandler(validatorChecker, feeCalculator)

	statedb.AddBalance(nonValidatorAddr, uint256.NewInt(1000000000000000000), tracing.BalanceChangeUnspecified)

	initialBalance := statedb.GetBalance(nonValidatorAddr)

	err := chargeContractDeploymentFeeIfNeeded(evm, nonValidatorAddr, statedb, nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Balance should not change when total supply is negative
	finalBalance := statedb.GetBalance(nonValidatorAddr)
	if finalBalance.Cmp(initialBalance) != 0 {
		t.Errorf("Balance should not change when total supply is negative, initial: %v, final: %v", initialBalance, finalBalance)
	}
}

func TestChargeContractDeploymentFeeIfNeeded_ZeroFee(t *testing.T) {
	nonValidatorAddr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	feeReceiverAddr := common.HexToAddress("0x9999999999999999999999999999999999999999")

	validatorChecker := newMockValidatorChecker([]common.Address{})
	// Use a very small supply that results in zero fee after calculation
	feeCalculator := newMockFeeCalculator(big.NewInt(1), feeReceiverAddr)

	evm, statedb := createTestEVMForHandler(validatorChecker, feeCalculator)

	statedb.AddBalance(nonValidatorAddr, uint256.NewInt(1000000000000000000), tracing.BalanceChangeUnspecified)

	initialBalance := statedb.GetBalance(nonValidatorAddr)

	err := chargeContractDeploymentFeeIfNeeded(evm, nonValidatorAddr, statedb, nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Balance should not change when calculated fee is zero
	finalBalance := statedb.GetBalance(nonValidatorAddr)
	if finalBalance.Cmp(initialBalance) != 0 {
		t.Errorf("Balance should not change when fee is zero, initial: %v, final: %v", initialBalance, finalBalance)
	}
}

func TestChargeContractDeploymentFeeIfNeeded_FeeOverflow(t *testing.T) {
	nonValidatorAddr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	feeReceiverAddr := common.HexToAddress("0x9999999999999999999999999999999999999999")

	validatorChecker := newMockValidatorChecker([]common.Address{})
	// Use an extremely large supply that might cause overflow
	extremelyLargeSupply := new(big.Int).Exp(big.NewInt(10), big.NewInt(100), nil)
	feeCalculator := newMockFeeCalculator(extremelyLargeSupply, feeReceiverAddr)

	evm, statedb := createTestEVMForHandler(validatorChecker, feeCalculator)

	statedb.AddBalance(nonValidatorAddr, uint256.NewInt(1000000000000000000), tracing.BalanceChangeUnspecified)

	err := chargeContractDeploymentFeeIfNeeded(evm, nonValidatorAddr, statedb, nil)

	// Should handle overflow gracefully
	if err != nil {
		// Error is expected for overflow case
		if !strings.HasPrefix(err.Error(), "extra fee overflow") {
			t.Errorf("Expected overflow error, got: %v", err)
		}
	}
}

func TestChargeContractDeploymentFeeIfNeeded_ExactBalance(t *testing.T) {
	nonValidatorAddr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	feeReceiverAddr := common.HexToAddress("0x9999999999999999999999999999999999999999")

	validatorChecker := newMockValidatorChecker([]common.Address{})
	totalSupply, _ := new(big.Int).SetString("1000000000000000000000000000", 10)
	feeCalculator := newMockFeeCalculator(totalSupply, feeReceiverAddr)

	evm, statedb := createTestEVMForHandler(validatorChecker, feeCalculator)

	// Set exact balance (exactly the fee amount)
	expectedFee := feeCalculator.CalculateFee(totalSupply)
	statedb.AddBalance(nonValidatorAddr, uint256.MustFromBig(expectedFee), tracing.BalanceChangeUnspecified)

	initialFeeReceiverBalance := statedb.GetBalance(feeReceiverAddr)

	err := chargeContractDeploymentFeeIfNeeded(evm, nonValidatorAddr, statedb, nil)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Balance should be zero after fee
	finalBalance := statedb.GetBalance(nonValidatorAddr)
	if finalBalance.Sign() != 0 {
		t.Errorf("Balance should be zero after paying exact fee, got: %v", finalBalance)
	}

	// Fee receiver should receive the fee
	finalFeeReceiverBalance := statedb.GetBalance(feeReceiverAddr)
	expectedFeeReceiverBalance := new(big.Int).Add(initialFeeReceiverBalance.ToBig(), expectedFee)
	if finalFeeReceiverBalance.Cmp(uint256.MustFromBig(expectedFeeReceiverBalance)) != 0 {
		t.Errorf("Fee receiver balance incorrect, expected: %v, got: %v", expectedFeeReceiverBalance, finalFeeReceiverBalance)
	}
}

func TestChargeContractDeploymentFeeIfNeeded_MultipleNonValidators(t *testing.T) {
	validatorAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")
	nonValidator1 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	nonValidator2 := common.HexToAddress("0x3333333333333333333333333333333333333333")
	feeReceiverAddr := common.HexToAddress("0x9999999999999999999999999999999999999999")

	validatorChecker := newMockValidatorChecker([]common.Address{validatorAddr})
	totalSupply, _ := new(big.Int).SetString("1000000000000000000000000000", 10)
	feeCalculator := newMockFeeCalculator(totalSupply, feeReceiverAddr)

	evm, statedb := createTestEVMForHandler(validatorChecker, feeCalculator)

	expectedFee := feeCalculator.CalculateFee(totalSupply)
	sufficientBalance := new(big.Int).Mul(expectedFee, big.NewInt(2))

	// Charge fee for first non-validator
	statedb.AddBalance(nonValidator1, uint256.MustFromBig(sufficientBalance), tracing.BalanceChangeUnspecified)
	err1 := chargeContractDeploymentFeeIfNeeded(evm, nonValidator1, statedb, nil)
	if err1 != nil {
		t.Errorf("Unexpected error for nonValidator1: %v", err1)
	}

	// Charge fee for second non-validator
	statedb.AddBalance(nonValidator2, uint256.MustFromBig(sufficientBalance), tracing.BalanceChangeUnspecified)
	err2 := chargeContractDeploymentFeeIfNeeded(evm, nonValidator2, statedb, nil)
	if err2 != nil {
		t.Errorf("Unexpected error for nonValidator2: %v", err2)
	}

	// Both should be charged
	balance1 := statedb.GetBalance(nonValidator1)
	balance2 := statedb.GetBalance(nonValidator2)
	expectedBalance := new(big.Int).Sub(sufficientBalance, expectedFee)

	if balance1.Cmp(uint256.MustFromBig(expectedBalance)) != 0 {
		t.Errorf("NonValidator1 balance incorrect, expected: %v, got: %v", expectedBalance, balance1)
	}

	if balance2.Cmp(uint256.MustFromBig(expectedBalance)) != 0 {
		t.Errorf("NonValidator2 balance incorrect, expected: %v, got: %v", expectedBalance, balance2)
	}

	// Fee receiver should receive both fees
	feeReceiverBalance := statedb.GetBalance(feeReceiverAddr)
	expectedTotalFees := new(big.Int).Mul(expectedFee, big.NewInt(2))
	if feeReceiverBalance.Cmp(uint256.MustFromBig(expectedTotalFees)) < 0 {
		t.Errorf("Fee receiver should receive both fees, expected at least: %v, got: %v", expectedTotalFees, feeReceiverBalance)
	}
}

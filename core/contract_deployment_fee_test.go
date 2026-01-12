package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

func createTestEVMForFee() (*vm.EVM, *state.StateDB) {
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
	evm := vm.NewEVM(blockCtx, statedb, params.TestChainConfig, vm.Config{})

	return evm, statedb
}

func TestNewContractDeploymentFeeCalculator(t *testing.T) {
	tests := []struct {
		name                   string
		feePercentage          *big.Int
		totalSupplyStorageAddr common.Address
		totalSupplyStorageSlot common.Hash
		expectedFeePercentage  int64
	}{
		{
			name:                   "with fee percentage",
			feePercentage:          big.NewInt(100),
			totalSupplyStorageAddr: common.HexToAddress("0x1111111111111111111111111111111111111111"),
			totalSupplyStorageSlot: common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000"),
			expectedFeePercentage:  100,
		},
		{
			name:                   "nil fee percentage uses default",
			feePercentage:          nil,
			totalSupplyStorageAddr: common.HexToAddress("0x2222222222222222222222222222222222222222"),
			totalSupplyStorageSlot: common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001"),
			expectedFeePercentage:  DefaultFeePercentage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := NewContractDeploymentFeeCalculator(
				tt.feePercentage,
				tt.totalSupplyStorageAddr,
				tt.totalSupplyStorageSlot,
			)

			if calc == nil {
				t.Fatal("NewContractDeploymentFeeCalculator returned nil")
			}

			if calc.feePercentage.Int64() != tt.expectedFeePercentage {
				t.Errorf("Expected fee percentage %d, got %d", tt.expectedFeePercentage, calc.feePercentage.Int64())
			}

			if calc.totalSupplyStorageAddr != tt.totalSupplyStorageAddr {
				t.Errorf("Expected storage address %v, got %v", tt.totalSupplyStorageAddr, calc.totalSupplyStorageAddr)
			}

			if calc.totalSupplyStorageSlot != tt.totalSupplyStorageSlot {
				t.Errorf("Expected storage slot %v, got %v", tt.totalSupplyStorageSlot, calc.totalSupplyStorageSlot)
			}
		})
	}
}

func TestCalculateFee(t *testing.T) {
	calc := NewContractDeploymentFeeCalculator(
		big.NewInt(100), // 1% (100/10000)
		common.Address{},
		common.Hash{},
	)

	tests := []struct {
		name         string
		totalSupply  *big.Int
		expectedFee  *big.Int
		shouldBeZero bool
	}{
		{
			name: "normal calculation",
			totalSupply: func() *big.Int {
				v, _ := new(big.Int).SetString("1000000000000000000000000000", 10)
				return v
			}(), // 1e27
			expectedFee: func() *big.Int {
				v, _ := new(big.Int).SetString("10000000000000000000000000", 10)
				return v
			}(), // 1e25 (1% of 1e27)
			shouldBeZero: false,
		},
		{
			name:         "small supply",
			totalSupply:  big.NewInt(10000),
			expectedFee:  big.NewInt(100), // 10000 * 100 / 10000 = 100 (1% of 10000)
			shouldBeZero: false,
		},
		{
			name:         "nil total supply",
			totalSupply:  nil,
			expectedFee:  big.NewInt(0),
			shouldBeZero: true,
		},
		{
			name:         "zero total supply",
			totalSupply:  big.NewInt(0),
			expectedFee:  big.NewInt(0),
			shouldBeZero: true,
		},
		{
			name:         "negative total supply",
			totalSupply:  big.NewInt(-1000),
			expectedFee:  big.NewInt(0),
			shouldBeZero: true,
		},
		{
			name:         "very large supply",
			totalSupply:  new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil),
			expectedFee:  new(big.Int).Div(new(big.Int).Mul(new(big.Int).Exp(big.NewInt(10), big.NewInt(30), nil), big.NewInt(100)), big.NewInt(10000)),
			shouldBeZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fee := calc.CalculateFee(tt.totalSupply)

			if fee == nil {
				t.Fatal("CalculateFee returned nil")
			}

			if tt.shouldBeZero {
				if fee.Sign() != 0 {
					t.Errorf("Expected zero fee, got %v", fee)
				}
			} else {
				if fee.Sign() == 0 && tt.totalSupply != nil && tt.totalSupply.Sign() > 0 {
					// For non-zero supply, fee should be at least 1
					if tt.expectedFee.Cmp(big.NewInt(1)) >= 0 {
						t.Errorf("Expected non-zero fee, got %v", fee)
					}
				}

				if tt.expectedFee != nil && fee.Cmp(tt.expectedFee) != 0 {
					t.Errorf("Expected fee %v, got %v", tt.expectedFee, fee)
				}
			}
		})
	}
}

func TestCalculateFee_MinimumFee(t *testing.T) {
	calc := NewContractDeploymentFeeCalculator(
		big.NewInt(100),
		common.Address{},
		common.Hash{},
	)

	// Test that very small fees result in minimum fee of 1
	smallSupply := big.NewInt(50) // 50 * 100 / 10000 = 0.5, should round to 1
	fee := calc.CalculateFee(smallSupply)

	if fee.Cmp(big.NewInt(1)) != 0 {
		t.Errorf("Expected minimum fee of 1, got %v", fee)
	}
}

func TestGetTotalSupply_EmptyStorageAddress(t *testing.T) {
	calc := NewContractDeploymentFeeCalculator(
		big.NewInt(100),
		common.Address{}, // Empty address
		common.Hash{},
	)

	evm, _ := createTestEVMForFee()
	supply := calc.GetTotalSupply(evm)

	if supply == nil {
		t.Fatal("GetTotalSupply returned nil")
	}

	expected, _ := new(big.Int).SetString("1000000000000000000000000000", 10)
	if supply.Cmp(expected) != 0 {
		t.Errorf("Expected default supply %v, got %v", expected, supply)
	}
}

func TestGetTotalSupply_WithStorageAddress(t *testing.T) {
	evm, statedb := createTestEVMForFee()

	storageAddr := common.HexToAddress("0x1111111111111111111111111111111111111111")
	storageSlot := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")

	calc := NewContractDeploymentFeeCalculator(
		big.NewInt(100),
		storageAddr,
		storageSlot,
	)

	// Set storage value
	expectedSupply, _ := new(big.Int).SetString("2000000000000000000000000000", 10) // 2e27
	var supplyBytes [32]byte
	copy(supplyBytes[32-len(expectedSupply.Bytes()):], expectedSupply.Bytes())
	statedb.SetState(storageAddr, storageSlot, common.BytesToHash(supplyBytes[:]))

	supply := calc.GetTotalSupply(evm)

	if supply == nil {
		t.Fatal("GetTotalSupply returned nil")
	}

	if supply.Cmp(expectedSupply) != 0 {
		t.Errorf("Expected supply %v, got %v", expectedSupply, supply)
	}
}

func TestGetTotalSupply_ZeroStorageValue(t *testing.T) {
	evm, statedb := createTestEVMForFee()

	storageAddr := common.HexToAddress("0x2222222222222222222222222222222222222222")
	storageSlot := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")

	calc := NewContractDeploymentFeeCalculator(
		big.NewInt(100),
		storageAddr,
		storageSlot,
	)

	// Set zero storage value
	statedb.SetState(storageAddr, storageSlot, common.Hash{})

	supply := calc.GetTotalSupply(evm)

	if supply == nil {
		t.Fatal("GetTotalSupply returned nil")
	}

	if supply.Sign() != 0 {
		t.Errorf("Expected zero supply, got %v", supply)
	}
}

func TestGetTotalSupply_DifferentStorageSlots(t *testing.T) {
	evm, statedb := createTestEVMForFee()

	storageAddr := common.HexToAddress("0x3333333333333333333333333333333333333333")
	slot1 := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000000")
	slot2 := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001")

	calc1 := NewContractDeploymentFeeCalculator(big.NewInt(100), storageAddr, slot1)
	calc2 := NewContractDeploymentFeeCalculator(big.NewInt(100), storageAddr, slot2)

	supply1, _ := new(big.Int).SetString("1000000000000000000000000000", 10)
	supply2, _ := new(big.Int).SetString("2000000000000000000000000000", 10)

	var bytes1, bytes2 [32]byte
	copy(bytes1[32-len(supply1.Bytes()):], supply1.Bytes())
	copy(bytes2[32-len(supply2.Bytes()):], supply2.Bytes())

	statedb.SetState(storageAddr, slot1, common.BytesToHash(bytes1[:]))
	statedb.SetState(storageAddr, slot2, common.BytesToHash(bytes2[:]))

	result1 := calc1.GetTotalSupply(evm)
	result2 := calc2.GetTotalSupply(evm)

	if result1.Cmp(supply1) != 0 {
		t.Errorf("Expected supply1 %v, got %v", supply1, result1)
	}

	if result2.Cmp(supply2) != 0 {
		t.Errorf("Expected supply2 %v, got %v", supply2, result2)
	}
}

func TestGetFeeReceiver(t *testing.T) {
	calc := NewContractDeploymentFeeCalculator(
		big.NewInt(100),
		common.Address{},
		common.Hash{},
	)

	receiver := calc.GetFeeReceiver()

	// ContractDeploymentFeeReceiver is set from FeeReceiverAddress constant
	// which is currently empty, so it should be zero address
	expected := common.HexToAddress("")
	if receiver != expected {
		t.Errorf("Expected receiver %v, got %v", expected, receiver)
	}
}

func TestCalculateFee_DifferentPercentages(t *testing.T) {
	tests := []struct {
		name        string
		percentage  *big.Int
		totalSupply *big.Int
		expectedFee *big.Int
	}{
		{
			name:        "1% fee",
			percentage:  big.NewInt(100),
			totalSupply: big.NewInt(1000000),
			expectedFee: big.NewInt(10000), // 1% of 1000000
		},
		{
			name:        "0.5% fee",
			percentage:  big.NewInt(50),
			totalSupply: big.NewInt(1000000),
			expectedFee: big.NewInt(5000), // 0.5% of 1000000
		},
		{
			name:        "2% fee",
			percentage:  big.NewInt(200),
			totalSupply: big.NewInt(1000000),
			expectedFee: big.NewInt(20000), // 2% of 1000000
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := NewContractDeploymentFeeCalculator(
				tt.percentage,
				common.Address{},
				common.Hash{},
			)

			fee := calc.CalculateFee(tt.totalSupply)

			if fee.Cmp(tt.expectedFee) != 0 {
				t.Errorf("Expected fee %v, got %v", tt.expectedFee, fee)
			}
		})
	}
}

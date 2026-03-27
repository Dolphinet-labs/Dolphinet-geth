package core

import (
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

// createTestEVM creates a test EVM instance with a state database
func createTestEVM() (*vm.EVM, *state.StateDB) {
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

// encodeAddressArray encodes an array of addresses in ABI format
func encodeAddressArray(addresses []common.Address) []byte {
	// Offset to the array data (32 bytes for offset)
	offset := uint64(32)
	length := uint64(len(addresses))
	dataLen := 32 + 32 + length*32 // offset + length + data

	data := make([]byte, dataLen)
	// Write offset (at position 24-32, big-endian)
	binary.BigEndian.PutUint64(data[24:32], offset)
	// Write length (at offset position)
	binary.BigEndian.PutUint64(data[offset+24:offset+32], length)
	// Write addresses
	for i, addr := range addresses {
		addrOffset := offset + 32 + uint64(i)*32
		copy(data[addrOffset+12:addrOffset+32], addr.Bytes())
	}

	return data
}

// createMockValidatorContract creates a mock contract that returns validator addresses
func createMockValidatorContract(statedb *state.StateDB, contractAddr common.Address, validators []common.Address) {
	// Create the contract account
	statedb.CreateAccount(contractAddr)
	statedb.SetCode(contractAddr, []byte{0x60, 0x00, 0x60, 0x00, 0x52}) // Simple return contract

	// For testing, we'll need to mock the StaticCall result
	// Since we can't easily mock StaticCall, we'll test the decoding logic separately
}

func TestNewContractValidatorChecker(t *testing.T) {
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	checker := NewContractValidatorChecker(contractAddr)

	if checker == nil {
		t.Fatal("NewContractValidatorChecker returned nil")
	}

	if checker.validatorContractAddr != contractAddr {
		t.Errorf("Expected contract address %v, got %v", contractAddr, checker.validatorContractAddr)
	}
}

func TestGetValidatorsMethodID(t *testing.T) {
	methodID := getValidatorsMethodID()
	expected := crypto.Keccak256([]byte("getValidators()"))[:4]

	if len(methodID) != 4 {
		t.Errorf("Expected method ID length 4, got %d", len(methodID))
	}

	if string(methodID) != string(expected) {
		t.Errorf("Expected method ID %x, got %x", expected, methodID)
	}
}

func TestDecodeAddressArray(t *testing.T) {
	tests := []struct {
		name      string
		addresses []common.Address
		wantErr   bool
	}{
		{
			name:      "empty array",
			addresses: []common.Address{},
			wantErr:   false,
		},
		{
			name: "single address",
			addresses: []common.Address{
				common.HexToAddress("0x1111111111111111111111111111111111111111"),
			},
			wantErr: false,
		},
		{
			name: "multiple addresses",
			addresses: []common.Address{
				common.HexToAddress("0x1111111111111111111111111111111111111111"),
				common.HexToAddress("0x2222222222222222222222222222222222222222"),
				common.HexToAddress("0x3333333333333333333333333333333333333333"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := encodeAddressArray(tt.addresses)
			decoded, err := decodeAddressArray(encoded)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(decoded) != len(tt.addresses) {
				t.Errorf("Expected %d addresses, got %d", len(tt.addresses), len(decoded))
				return
			}

			for i, addr := range tt.addresses {
				if decoded[i] != addr {
					t.Errorf("Address %d: expected %v, got %v", i, addr, decoded[i])
				}
			}
		})
	}
}

func TestDecodeAddressArray_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "insufficient data",
			data:    []byte{0x00, 0x01},
			wantErr: true,
		},
		{
			name:    "offset out of bounds",
			data:    make([]byte, 64),
			wantErr: true,
		},
		{
			name: "length out of bounds",
			data: func() []byte {
				// Create 64 bytes with offset = 33 (valid offset but offset+32 = 65 would be out of bounds)
				data := make([]byte, 64)
				// Set offset to 33 at position 24-32
				binary.BigEndian.PutUint64(data[24:32], 33)
				return data
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeAddressArray(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Expected error: %v, got: %v", tt.wantErr, err != nil)
			}
		})
	}
}

func TestIsValidator_EmptyContractAddress(t *testing.T) {
	evm, _ := createTestEVM()
	checker := NewContractValidatorChecker(common.Address{})
	addr := common.HexToAddress("0x1111111111111111111111111111111111111111")

	result := checker.IsValidator(addr, evm)
	if result {
		t.Error("Expected false for empty contract address")
	}
}

func TestIsValidator_ContractNotExist(t *testing.T) {
	evm, _ := createTestEVM()
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	checker := NewContractValidatorChecker(contractAddr)
	addr := common.HexToAddress("0x1111111111111111111111111111111111111111")

	result := checker.IsValidator(addr, evm)
	if result {
		t.Error("Expected false when contract does not exist")
	}
}

func TestIsValidator_ContractExistsButEmptyResult(t *testing.T) {
	evm, statedb := createTestEVM()
	contractAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	checker := NewContractValidatorChecker(contractAddr)

	// Create contract account but without proper code to return validators
	statedb.CreateAccount(contractAddr)
	statedb.SetCode(contractAddr, []byte{0x60, 0x00, 0x60, 0x00, 0x52}) // Simple return empty

	addr := common.HexToAddress("0x1111111111111111111111111111111111111111")

	result := checker.IsValidator(addr, evm)
	if result {
		t.Error("Expected false when contract returns empty result")
	}
}

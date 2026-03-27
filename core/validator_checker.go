package core

import (
	"encoding/binary"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

type ValidatorChecker interface {
	IsValidator(addr common.Address, evm *vm.EVM) bool
}

type ContractValidatorChecker struct {
	validatorContractAddr common.Address
}

func NewContractValidatorChecker(contractAddr common.Address) *ContractValidatorChecker {
	checker := &ContractValidatorChecker{
		validatorContractAddr: contractAddr,
	}
	log.Info("NewContractValidatorChecker created", "contract", contractAddr.Hex())
	return checker
}

func getValidatorsMethodID() []byte {
	signature := []byte("getValidators()")
	hash := crypto.Keccak256(signature)
	return hash[:4]
}

func decodeAddressArray(data []byte) ([]common.Address, error) {
	if len(data) < 64 {
		return nil, fmt.Errorf("insufficient data for array: got %d bytes, need at least 64", len(data))
	}

	offset := binary.BigEndian.Uint64(data[24:32])
	// Offset should be at least 32 (pointing to data after the offset field)
	if offset < 32 {
		return nil, fmt.Errorf("offset out of bounds: %d < 32", offset)
	}
	if offset >= uint64(len(data)) {
		return nil, fmt.Errorf("offset out of bounds: %d >= %d", offset, len(data))
	}

	if offset+32 > uint64(len(data)) {
		return nil, fmt.Errorf("length out of bounds: offset %d + 32 > %d", offset, len(data))
	}
	length := binary.BigEndian.Uint64(data[offset+24 : offset+32])

	requiredLen := offset + 32 + length*32
	if uint64(len(data)) < requiredLen {
		return nil, fmt.Errorf("insufficient data for array elements: need %d bytes, got %d", requiredLen, len(data))
	}

	addresses := make([]common.Address, length)
	for i := uint64(0); i < length; i++ {
		addrOffset := offset + 32 + i*32
		addresses[i] = common.BytesToAddress(data[addrOffset+12 : addrOffset+32])
	}

	return addresses, nil
}

func (c *ContractValidatorChecker) IsValidator(addr common.Address, evm *vm.EVM) bool {
	log.Debug("IsValidator called",
		"addr", addr.Hex(),
		"contract", c.validatorContractAddr.Hex(),
	)
	if c.validatorContractAddr == (common.Address{}) {
		log.Debug("Validator contract address not configured, treating as non-validator",
			"addr", addr.Hex(),
			"contract", c.validatorContractAddr.Hex(),
		)
		return false
	}

	if !evm.StateDB.Exist(c.validatorContractAddr) {
		log.Debug("Validator contract does not exist",
			"contract", c.validatorContractAddr.Hex(),
			"addr", addr.Hex(),
		)
		return false
	}

	callData := getValidatorsMethodID()

	ret, _, err := evm.StaticCall(
		common.Address{},
		c.validatorContractAddr,
		callData,
		100000,
	)

	if err != nil {
		log.Debug("Failed to call getValidators",
			"contract", c.validatorContractAddr.Hex(),
			"err", err,
			"addr", addr.Hex(),
		)
		return false
	}

	if len(ret) == 0 {
		log.Debug("getValidators returned empty result",
			"contract", c.validatorContractAddr.Hex(),
			"addr", addr.Hex(),
		)
		return false
	}

	validators, err := decodeAddressArray(ret)
	if err != nil {
		log.Warn("Failed to decode validator list",
			"contract", c.validatorContractAddr.Hex(),
			"err", err,
			"addr", addr.Hex(),
		)
		return false
	}

	for _, validator := range validators {
		if validator == addr {
			log.Debug("Address is a validator",
				"addr", addr.Hex(),
				"contract", c.validatorContractAddr.Hex(),
			)
			return true
		}
	}

	log.Debug("Address is not a validator",
		"addr", addr.Hex(),
		"contract", c.validatorContractAddr.Hex(),
		"validatorCount", len(validators),
	)
	return false
}

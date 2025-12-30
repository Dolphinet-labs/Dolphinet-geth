package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
)

type ValidatorChecker interface {
	IsValidator(addr common.Address, evm *vm.EVM) bool
}

type ContractValidatorChecker struct {
	validatorContractAddr common.Address
}

func NewContractValidatorChecker(contractAddr common.Address) *ContractValidatorChecker {
	return &ContractValidatorChecker{
		validatorContractAddr: contractAddr,
	}
}

func (c *ContractValidatorChecker) IsValidator(addr common.Address, evm *vm.EVM) bool {
	if c.validatorContractAddr == (common.Address{}) {
		return false
	}

	// TODO: adjust pos contracts

	log.Debug("Validator contract not implemented yet, treating all users as non-validators", "addr", addr.Hex())
	return false
}

// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package bindings

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// CreditsMetaData contains all meta data concerning the Credits contract.
var CreditsMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"OwnableInvalidOwner\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"OwnableUnauthorizedAccount\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"fromAddr\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"toAddr\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"CreditsBought\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"CreditsStaked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"CreditsUnstaked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"parameterControllerAddress\",\"type\":\"address\"}],\"name\":\"ParameterControllerUpdated\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"createCredits\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getAllCreditAddresses\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getAllCredits\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"getCredits\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"setAdminAddress\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"setParameterController\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"setStakingAddress\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"stakeCredits\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"unstakeCredits\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// CreditsABI is the input ABI used to generate the binding from.
// Deprecated: Use CreditsMetaData.ABI instead.
var CreditsABI = CreditsMetaData.ABI

// Credits is an auto generated Go binding around an Ethereum contract.
type Credits struct {
	CreditsCaller     // Read-only binding to the contract
	CreditsTransactor // Write-only binding to the contract
	CreditsFilterer   // Log filterer for contract events
}

// CreditsCaller is an auto generated read-only Go binding around an Ethereum contract.
type CreditsCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CreditsTransactor is an auto generated write-only Go binding around an Ethereum contract.
type CreditsTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CreditsFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type CreditsFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CreditsSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type CreditsSession struct {
	Contract     *Credits          // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// CreditsCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type CreditsCallerSession struct {
	Contract *CreditsCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts  // Call options to use throughout this session
}

// CreditsTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type CreditsTransactorSession struct {
	Contract     *CreditsTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// CreditsRaw is an auto generated low-level Go binding around an Ethereum contract.
type CreditsRaw struct {
	Contract *Credits // Generic contract binding to access the raw methods on
}

// CreditsCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type CreditsCallerRaw struct {
	Contract *CreditsCaller // Generic read-only contract binding to access the raw methods on
}

// CreditsTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type CreditsTransactorRaw struct {
	Contract *CreditsTransactor // Generic write-only contract binding to access the raw methods on
}

// NewCredits creates a new instance of Credits, bound to a specific deployed contract.
func NewCredits(address common.Address, backend bind.ContractBackend) (*Credits, error) {
	contract, err := bindCredits(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Credits{CreditsCaller: CreditsCaller{contract: contract}, CreditsTransactor: CreditsTransactor{contract: contract}, CreditsFilterer: CreditsFilterer{contract: contract}}, nil
}

// NewCreditsCaller creates a new read-only instance of Credits, bound to a specific deployed contract.
func NewCreditsCaller(address common.Address, caller bind.ContractCaller) (*CreditsCaller, error) {
	contract, err := bindCredits(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &CreditsCaller{contract: contract}, nil
}

// NewCreditsTransactor creates a new write-only instance of Credits, bound to a specific deployed contract.
func NewCreditsTransactor(address common.Address, transactor bind.ContractTransactor) (*CreditsTransactor, error) {
	contract, err := bindCredits(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &CreditsTransactor{contract: contract}, nil
}

// NewCreditsFilterer creates a new log filterer instance of Credits, bound to a specific deployed contract.
func NewCreditsFilterer(address common.Address, filterer bind.ContractFilterer) (*CreditsFilterer, error) {
	contract, err := bindCredits(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &CreditsFilterer{contract: contract}, nil
}

// bindCredits binds a generic wrapper to an already deployed contract.
func bindCredits(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := CreditsMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Credits *CreditsRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Credits.Contract.CreditsCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Credits *CreditsRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Credits.Contract.CreditsTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Credits *CreditsRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Credits.Contract.CreditsTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Credits *CreditsCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Credits.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Credits *CreditsTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Credits.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Credits *CreditsTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Credits.Contract.contract.Transact(opts, method, params...)
}

// GetAllCreditAddresses is a free data retrieval call binding the contract method 0x17528e03.
//
// Solidity: function getAllCreditAddresses() view returns(address[])
func (_Credits *CreditsCaller) GetAllCreditAddresses(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _Credits.contract.Call(opts, &out, "getAllCreditAddresses")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetAllCreditAddresses is a free data retrieval call binding the contract method 0x17528e03.
//
// Solidity: function getAllCreditAddresses() view returns(address[])
func (_Credits *CreditsSession) GetAllCreditAddresses() ([]common.Address, error) {
	return _Credits.Contract.GetAllCreditAddresses(&_Credits.CallOpts)
}

// GetAllCreditAddresses is a free data retrieval call binding the contract method 0x17528e03.
//
// Solidity: function getAllCreditAddresses() view returns(address[])
func (_Credits *CreditsCallerSession) GetAllCreditAddresses() ([]common.Address, error) {
	return _Credits.Contract.GetAllCreditAddresses(&_Credits.CallOpts)
}

// GetAllCredits is a free data retrieval call binding the contract method 0x2861072e.
//
// Solidity: function getAllCredits() view returns(address[], uint256[])
func (_Credits *CreditsCaller) GetAllCredits(opts *bind.CallOpts) ([]common.Address, []*big.Int, error) {
	var out []interface{}
	err := _Credits.contract.Call(opts, &out, "getAllCredits")

	if err != nil {
		return *new([]common.Address), *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)
	out1 := *abi.ConvertType(out[1], new([]*big.Int)).(*[]*big.Int)

	return out0, out1, err

}

// GetAllCredits is a free data retrieval call binding the contract method 0x2861072e.
//
// Solidity: function getAllCredits() view returns(address[], uint256[])
func (_Credits *CreditsSession) GetAllCredits() ([]common.Address, []*big.Int, error) {
	return _Credits.Contract.GetAllCredits(&_Credits.CallOpts)
}

// GetAllCredits is a free data retrieval call binding the contract method 0x2861072e.
//
// Solidity: function getAllCredits() view returns(address[], uint256[])
func (_Credits *CreditsCallerSession) GetAllCredits() ([]common.Address, []*big.Int, error) {
	return _Credits.Contract.GetAllCredits(&_Credits.CallOpts)
}

// GetCredits is a free data retrieval call binding the contract method 0x3b66e9f6.
//
// Solidity: function getCredits(address addr) view returns(uint256)
func (_Credits *CreditsCaller) GetCredits(opts *bind.CallOpts, addr common.Address) (*big.Int, error) {
	var out []interface{}
	err := _Credits.contract.Call(opts, &out, "getCredits", addr)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetCredits is a free data retrieval call binding the contract method 0x3b66e9f6.
//
// Solidity: function getCredits(address addr) view returns(uint256)
func (_Credits *CreditsSession) GetCredits(addr common.Address) (*big.Int, error) {
	return _Credits.Contract.GetCredits(&_Credits.CallOpts, addr)
}

// GetCredits is a free data retrieval call binding the contract method 0x3b66e9f6.
//
// Solidity: function getCredits(address addr) view returns(uint256)
func (_Credits *CreditsCallerSession) GetCredits(addr common.Address) (*big.Int, error) {
	return _Credits.Contract.GetCredits(&_Credits.CallOpts, addr)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Credits *CreditsCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Credits.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Credits *CreditsSession) Owner() (common.Address, error) {
	return _Credits.Contract.Owner(&_Credits.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Credits *CreditsCallerSession) Owner() (common.Address, error) {
	return _Credits.Contract.Owner(&_Credits.CallOpts)
}

// CreateCredits is a paid mutator transaction binding the contract method 0x63307643.
//
// Solidity: function createCredits(address addr, uint256 amount) returns()
func (_Credits *CreditsTransactor) CreateCredits(opts *bind.TransactOpts, addr common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Credits.contract.Transact(opts, "createCredits", addr, amount)
}

// CreateCredits is a paid mutator transaction binding the contract method 0x63307643.
//
// Solidity: function createCredits(address addr, uint256 amount) returns()
func (_Credits *CreditsSession) CreateCredits(addr common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Credits.Contract.CreateCredits(&_Credits.TransactOpts, addr, amount)
}

// CreateCredits is a paid mutator transaction binding the contract method 0x63307643.
//
// Solidity: function createCredits(address addr, uint256 amount) returns()
func (_Credits *CreditsTransactorSession) CreateCredits(addr common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Credits.Contract.CreateCredits(&_Credits.TransactOpts, addr, amount)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Credits *CreditsTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Credits.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Credits *CreditsSession) RenounceOwnership() (*types.Transaction, error) {
	return _Credits.Contract.RenounceOwnership(&_Credits.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Credits *CreditsTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _Credits.Contract.RenounceOwnership(&_Credits.TransactOpts)
}

// SetAdminAddress is a paid mutator transaction binding the contract method 0x2c1e816d.
//
// Solidity: function setAdminAddress(address addr) returns()
func (_Credits *CreditsTransactor) SetAdminAddress(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _Credits.contract.Transact(opts, "setAdminAddress", addr)
}

// SetAdminAddress is a paid mutator transaction binding the contract method 0x2c1e816d.
//
// Solidity: function setAdminAddress(address addr) returns()
func (_Credits *CreditsSession) SetAdminAddress(addr common.Address) (*types.Transaction, error) {
	return _Credits.Contract.SetAdminAddress(&_Credits.TransactOpts, addr)
}

// SetAdminAddress is a paid mutator transaction binding the contract method 0x2c1e816d.
//
// Solidity: function setAdminAddress(address addr) returns()
func (_Credits *CreditsTransactorSession) SetAdminAddress(addr common.Address) (*types.Transaction, error) {
	return _Credits.Contract.SetAdminAddress(&_Credits.TransactOpts, addr)
}

// SetParameterController is a paid mutator transaction binding the contract method 0xa0152dde.
//
// Solidity: function setParameterController(address addr) returns()
func (_Credits *CreditsTransactor) SetParameterController(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _Credits.contract.Transact(opts, "setParameterController", addr)
}

// SetParameterController is a paid mutator transaction binding the contract method 0xa0152dde.
//
// Solidity: function setParameterController(address addr) returns()
func (_Credits *CreditsSession) SetParameterController(addr common.Address) (*types.Transaction, error) {
	return _Credits.Contract.SetParameterController(&_Credits.TransactOpts, addr)
}

// SetParameterController is a paid mutator transaction binding the contract method 0xa0152dde.
//
// Solidity: function setParameterController(address addr) returns()
func (_Credits *CreditsTransactorSession) SetParameterController(addr common.Address) (*types.Transaction, error) {
	return _Credits.Contract.SetParameterController(&_Credits.TransactOpts, addr)
}

// SetStakingAddress is a paid mutator transaction binding the contract method 0xf4e0d9ac.
//
// Solidity: function setStakingAddress(address addr) returns()
func (_Credits *CreditsTransactor) SetStakingAddress(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _Credits.contract.Transact(opts, "setStakingAddress", addr)
}

// SetStakingAddress is a paid mutator transaction binding the contract method 0xf4e0d9ac.
//
// Solidity: function setStakingAddress(address addr) returns()
func (_Credits *CreditsSession) SetStakingAddress(addr common.Address) (*types.Transaction, error) {
	return _Credits.Contract.SetStakingAddress(&_Credits.TransactOpts, addr)
}

// SetStakingAddress is a paid mutator transaction binding the contract method 0xf4e0d9ac.
//
// Solidity: function setStakingAddress(address addr) returns()
func (_Credits *CreditsTransactorSession) SetStakingAddress(addr common.Address) (*types.Transaction, error) {
	return _Credits.Contract.SetStakingAddress(&_Credits.TransactOpts, addr)
}

// StakeCredits is a paid mutator transaction binding the contract method 0xd421b950.
//
// Solidity: function stakeCredits(address addr, uint256 amount) returns()
func (_Credits *CreditsTransactor) StakeCredits(opts *bind.TransactOpts, addr common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Credits.contract.Transact(opts, "stakeCredits", addr, amount)
}

// StakeCredits is a paid mutator transaction binding the contract method 0xd421b950.
//
// Solidity: function stakeCredits(address addr, uint256 amount) returns()
func (_Credits *CreditsSession) StakeCredits(addr common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Credits.Contract.StakeCredits(&_Credits.TransactOpts, addr, amount)
}

// StakeCredits is a paid mutator transaction binding the contract method 0xd421b950.
//
// Solidity: function stakeCredits(address addr, uint256 amount) returns()
func (_Credits *CreditsTransactorSession) StakeCredits(addr common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Credits.Contract.StakeCredits(&_Credits.TransactOpts, addr, amount)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Credits *CreditsTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _Credits.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Credits *CreditsSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Credits.Contract.TransferOwnership(&_Credits.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Credits *CreditsTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Credits.Contract.TransferOwnership(&_Credits.TransactOpts, newOwner)
}

// UnstakeCredits is a paid mutator transaction binding the contract method 0xeac0c954.
//
// Solidity: function unstakeCredits(address addr, uint256 amount) returns()
func (_Credits *CreditsTransactor) UnstakeCredits(opts *bind.TransactOpts, addr common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Credits.contract.Transact(opts, "unstakeCredits", addr, amount)
}

// UnstakeCredits is a paid mutator transaction binding the contract method 0xeac0c954.
//
// Solidity: function unstakeCredits(address addr, uint256 amount) returns()
func (_Credits *CreditsSession) UnstakeCredits(addr common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Credits.Contract.UnstakeCredits(&_Credits.TransactOpts, addr, amount)
}

// UnstakeCredits is a paid mutator transaction binding the contract method 0xeac0c954.
//
// Solidity: function unstakeCredits(address addr, uint256 amount) returns()
func (_Credits *CreditsTransactorSession) UnstakeCredits(addr common.Address, amount *big.Int) (*types.Transaction, error) {
	return _Credits.Contract.UnstakeCredits(&_Credits.TransactOpts, addr, amount)
}

// CreditsCreditsBoughtIterator is returned from FilterCreditsBought and is used to iterate over the raw logs and unpacked data for CreditsBought events raised by the Credits contract.
type CreditsCreditsBoughtIterator struct {
	Event *CreditsCreditsBought // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CreditsCreditsBoughtIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CreditsCreditsBought)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CreditsCreditsBought)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CreditsCreditsBoughtIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CreditsCreditsBoughtIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CreditsCreditsBought represents a CreditsBought event raised by the Credits contract.
type CreditsCreditsBought struct {
	FromAddr common.Address
	ToAddr   common.Address
	Amount   *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterCreditsBought is a free log retrieval operation binding the contract event 0xabb50c126402c62d6fdf8bbf2554cc904614e9ba3bf9e18b6b946f196854cf99.
//
// Solidity: event CreditsBought(address indexed fromAddr, address indexed toAddr, uint256 amount)
func (_Credits *CreditsFilterer) FilterCreditsBought(opts *bind.FilterOpts, fromAddr []common.Address, toAddr []common.Address) (*CreditsCreditsBoughtIterator, error) {

	var fromAddrRule []interface{}
	for _, fromAddrItem := range fromAddr {
		fromAddrRule = append(fromAddrRule, fromAddrItem)
	}
	var toAddrRule []interface{}
	for _, toAddrItem := range toAddr {
		toAddrRule = append(toAddrRule, toAddrItem)
	}

	logs, sub, err := _Credits.contract.FilterLogs(opts, "CreditsBought", fromAddrRule, toAddrRule)
	if err != nil {
		return nil, err
	}
	return &CreditsCreditsBoughtIterator{contract: _Credits.contract, event: "CreditsBought", logs: logs, sub: sub}, nil
}

// WatchCreditsBought is a free log subscription operation binding the contract event 0xabb50c126402c62d6fdf8bbf2554cc904614e9ba3bf9e18b6b946f196854cf99.
//
// Solidity: event CreditsBought(address indexed fromAddr, address indexed toAddr, uint256 amount)
func (_Credits *CreditsFilterer) WatchCreditsBought(opts *bind.WatchOpts, sink chan<- *CreditsCreditsBought, fromAddr []common.Address, toAddr []common.Address) (event.Subscription, error) {

	var fromAddrRule []interface{}
	for _, fromAddrItem := range fromAddr {
		fromAddrRule = append(fromAddrRule, fromAddrItem)
	}
	var toAddrRule []interface{}
	for _, toAddrItem := range toAddr {
		toAddrRule = append(toAddrRule, toAddrItem)
	}

	logs, sub, err := _Credits.contract.WatchLogs(opts, "CreditsBought", fromAddrRule, toAddrRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CreditsCreditsBought)
				if err := _Credits.contract.UnpackLog(event, "CreditsBought", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCreditsBought is a log parse operation binding the contract event 0xabb50c126402c62d6fdf8bbf2554cc904614e9ba3bf9e18b6b946f196854cf99.
//
// Solidity: event CreditsBought(address indexed fromAddr, address indexed toAddr, uint256 amount)
func (_Credits *CreditsFilterer) ParseCreditsBought(log types.Log) (*CreditsCreditsBought, error) {
	event := new(CreditsCreditsBought)
	if err := _Credits.contract.UnpackLog(event, "CreditsBought", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CreditsCreditsStakedIterator is returned from FilterCreditsStaked and is used to iterate over the raw logs and unpacked data for CreditsStaked events raised by the Credits contract.
type CreditsCreditsStakedIterator struct {
	Event *CreditsCreditsStaked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CreditsCreditsStakedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CreditsCreditsStaked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CreditsCreditsStaked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CreditsCreditsStakedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CreditsCreditsStakedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CreditsCreditsStaked represents a CreditsStaked event raised by the Credits contract.
type CreditsCreditsStaked struct {
	Addr   common.Address
	Amount *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterCreditsStaked is a free log retrieval operation binding the contract event 0x4ab889d95c92a5541c74b6406ec95729f6af442eca7c774846ccffaf816fa3fe.
//
// Solidity: event CreditsStaked(address indexed addr, uint256 amount)
func (_Credits *CreditsFilterer) FilterCreditsStaked(opts *bind.FilterOpts, addr []common.Address) (*CreditsCreditsStakedIterator, error) {

	var addrRule []interface{}
	for _, addrItem := range addr {
		addrRule = append(addrRule, addrItem)
	}

	logs, sub, err := _Credits.contract.FilterLogs(opts, "CreditsStaked", addrRule)
	if err != nil {
		return nil, err
	}
	return &CreditsCreditsStakedIterator{contract: _Credits.contract, event: "CreditsStaked", logs: logs, sub: sub}, nil
}

// WatchCreditsStaked is a free log subscription operation binding the contract event 0x4ab889d95c92a5541c74b6406ec95729f6af442eca7c774846ccffaf816fa3fe.
//
// Solidity: event CreditsStaked(address indexed addr, uint256 amount)
func (_Credits *CreditsFilterer) WatchCreditsStaked(opts *bind.WatchOpts, sink chan<- *CreditsCreditsStaked, addr []common.Address) (event.Subscription, error) {

	var addrRule []interface{}
	for _, addrItem := range addr {
		addrRule = append(addrRule, addrItem)
	}

	logs, sub, err := _Credits.contract.WatchLogs(opts, "CreditsStaked", addrRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CreditsCreditsStaked)
				if err := _Credits.contract.UnpackLog(event, "CreditsStaked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCreditsStaked is a log parse operation binding the contract event 0x4ab889d95c92a5541c74b6406ec95729f6af442eca7c774846ccffaf816fa3fe.
//
// Solidity: event CreditsStaked(address indexed addr, uint256 amount)
func (_Credits *CreditsFilterer) ParseCreditsStaked(log types.Log) (*CreditsCreditsStaked, error) {
	event := new(CreditsCreditsStaked)
	if err := _Credits.contract.UnpackLog(event, "CreditsStaked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CreditsCreditsUnstakedIterator is returned from FilterCreditsUnstaked and is used to iterate over the raw logs and unpacked data for CreditsUnstaked events raised by the Credits contract.
type CreditsCreditsUnstakedIterator struct {
	Event *CreditsCreditsUnstaked // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CreditsCreditsUnstakedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CreditsCreditsUnstaked)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CreditsCreditsUnstaked)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CreditsCreditsUnstakedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CreditsCreditsUnstakedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CreditsCreditsUnstaked represents a CreditsUnstaked event raised by the Credits contract.
type CreditsCreditsUnstaked struct {
	Addr   common.Address
	Amount *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterCreditsUnstaked is a free log retrieval operation binding the contract event 0x7b7b0f906696c7bb31db31e23c6a32b3d492fab9d6ed25904ca4490bb1e6841c.
//
// Solidity: event CreditsUnstaked(address indexed addr, uint256 amount)
func (_Credits *CreditsFilterer) FilterCreditsUnstaked(opts *bind.FilterOpts, addr []common.Address) (*CreditsCreditsUnstakedIterator, error) {

	var addrRule []interface{}
	for _, addrItem := range addr {
		addrRule = append(addrRule, addrItem)
	}

	logs, sub, err := _Credits.contract.FilterLogs(opts, "CreditsUnstaked", addrRule)
	if err != nil {
		return nil, err
	}
	return &CreditsCreditsUnstakedIterator{contract: _Credits.contract, event: "CreditsUnstaked", logs: logs, sub: sub}, nil
}

// WatchCreditsUnstaked is a free log subscription operation binding the contract event 0x7b7b0f906696c7bb31db31e23c6a32b3d492fab9d6ed25904ca4490bb1e6841c.
//
// Solidity: event CreditsUnstaked(address indexed addr, uint256 amount)
func (_Credits *CreditsFilterer) WatchCreditsUnstaked(opts *bind.WatchOpts, sink chan<- *CreditsCreditsUnstaked, addr []common.Address) (event.Subscription, error) {

	var addrRule []interface{}
	for _, addrItem := range addr {
		addrRule = append(addrRule, addrItem)
	}

	logs, sub, err := _Credits.contract.WatchLogs(opts, "CreditsUnstaked", addrRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CreditsCreditsUnstaked)
				if err := _Credits.contract.UnpackLog(event, "CreditsUnstaked", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseCreditsUnstaked is a log parse operation binding the contract event 0x7b7b0f906696c7bb31db31e23c6a32b3d492fab9d6ed25904ca4490bb1e6841c.
//
// Solidity: event CreditsUnstaked(address indexed addr, uint256 amount)
func (_Credits *CreditsFilterer) ParseCreditsUnstaked(log types.Log) (*CreditsCreditsUnstaked, error) {
	event := new(CreditsCreditsUnstaked)
	if err := _Credits.contract.UnpackLog(event, "CreditsUnstaked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CreditsOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the Credits contract.
type CreditsOwnershipTransferredIterator struct {
	Event *CreditsOwnershipTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CreditsOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CreditsOwnershipTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CreditsOwnershipTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CreditsOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CreditsOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CreditsOwnershipTransferred represents a OwnershipTransferred event raised by the Credits contract.
type CreditsOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Credits *CreditsFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*CreditsOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Credits.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &CreditsOwnershipTransferredIterator{contract: _Credits.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Credits *CreditsFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *CreditsOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Credits.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CreditsOwnershipTransferred)
				if err := _Credits.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Credits *CreditsFilterer) ParseOwnershipTransferred(log types.Log) (*CreditsOwnershipTransferred, error) {
	event := new(CreditsOwnershipTransferred)
	if err := _Credits.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// CreditsParameterControllerUpdatedIterator is returned from FilterParameterControllerUpdated and is used to iterate over the raw logs and unpacked data for ParameterControllerUpdated events raised by the Credits contract.
type CreditsParameterControllerUpdatedIterator struct {
	Event *CreditsParameterControllerUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *CreditsParameterControllerUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(CreditsParameterControllerUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(CreditsParameterControllerUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *CreditsParameterControllerUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *CreditsParameterControllerUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// CreditsParameterControllerUpdated represents a ParameterControllerUpdated event raised by the Credits contract.
type CreditsParameterControllerUpdated struct {
	ParameterControllerAddress common.Address
	Raw                        types.Log // Blockchain specific contextual infos
}

// FilterParameterControllerUpdated is a free log retrieval operation binding the contract event 0x4509827890b8a9ffbc5a343f5c719169da089858e3ba940cc3f33f4b95454ba3.
//
// Solidity: event ParameterControllerUpdated(address indexed parameterControllerAddress)
func (_Credits *CreditsFilterer) FilterParameterControllerUpdated(opts *bind.FilterOpts, parameterControllerAddress []common.Address) (*CreditsParameterControllerUpdatedIterator, error) {

	var parameterControllerAddressRule []interface{}
	for _, parameterControllerAddressItem := range parameterControllerAddress {
		parameterControllerAddressRule = append(parameterControllerAddressRule, parameterControllerAddressItem)
	}

	logs, sub, err := _Credits.contract.FilterLogs(opts, "ParameterControllerUpdated", parameterControllerAddressRule)
	if err != nil {
		return nil, err
	}
	return &CreditsParameterControllerUpdatedIterator{contract: _Credits.contract, event: "ParameterControllerUpdated", logs: logs, sub: sub}, nil
}

// WatchParameterControllerUpdated is a free log subscription operation binding the contract event 0x4509827890b8a9ffbc5a343f5c719169da089858e3ba940cc3f33f4b95454ba3.
//
// Solidity: event ParameterControllerUpdated(address indexed parameterControllerAddress)
func (_Credits *CreditsFilterer) WatchParameterControllerUpdated(opts *bind.WatchOpts, sink chan<- *CreditsParameterControllerUpdated, parameterControllerAddress []common.Address) (event.Subscription, error) {

	var parameterControllerAddressRule []interface{}
	for _, parameterControllerAddressItem := range parameterControllerAddress {
		parameterControllerAddressRule = append(parameterControllerAddressRule, parameterControllerAddressItem)
	}

	logs, sub, err := _Credits.contract.WatchLogs(opts, "ParameterControllerUpdated", parameterControllerAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(CreditsParameterControllerUpdated)
				if err := _Credits.contract.UnpackLog(event, "ParameterControllerUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseParameterControllerUpdated is a log parse operation binding the contract event 0x4509827890b8a9ffbc5a343f5c719169da089858e3ba940cc3f33f4b95454ba3.
//
// Solidity: event ParameterControllerUpdated(address indexed parameterControllerAddress)
func (_Credits *CreditsFilterer) ParseParameterControllerUpdated(log types.Log) (*CreditsParameterControllerUpdated, error) {
	event := new(CreditsParameterControllerUpdated)
	if err := _Credits.contract.UnpackLog(event, "ParameterControllerUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

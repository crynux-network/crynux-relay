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

// NodeStakingStakingInfo is an auto generated low-level Go binding around an user-defined struct.
type NodeStakingStakingInfo struct {
	NodeAddress      common.Address
	StakedBalance    *big.Int
	Status           uint8
	UnstakeTimestamp *big.Int
}

// NodeStakingMetaData contains all meta data concerning the NodeStaking contract.
var NodeStakingMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"benefitAddressContract\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"slashReceiverAddress\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"OwnableInvalidOwner\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"OwnableUnauthorizedAccount\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"OwnershipRenouncementDisabled\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ReentrancyGuardReentrantCall\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"oldAddress\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newAddress\",\"type\":\"address\"}],\"name\":\"AdminAddressUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"oldDelay\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newDelay\",\"type\":\"uint256\"}],\"name\":\"ForceUnstakeDelayUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"oldAmount\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newAmount\",\"type\":\"uint256\"}],\"name\":\"MinStakeAmountUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"stakedBalance\",\"type\":\"uint256\"}],\"name\":\"NodeSlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"stakedBalance\",\"type\":\"uint256\"}],\"name\":\"NodeStaked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"NodeTryUnstaked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"stakedBalance\",\"type\":\"uint256\"}],\"name\":\"NodeUnstaked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"oldObserver\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newObserver\",\"type\":\"address\"}],\"name\":\"ObserverUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"ba\",\"outputs\":[{\"internalType\":\"contractBenefitAddress\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"forceUnstake\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"page\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"pageSize\",\"type\":\"uint256\"}],\"name\":\"getAllNodeAddresses\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getForceUnstakeDelay\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getMinStakeAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"getStakingInfo\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"stakedBalance\",\"type\":\"uint256\"},{\"internalType\":\"enumNodeStaking.StakingStatus\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"uint256\",\"name\":\"unstakeTimestamp\",\"type\":\"uint256\"}],\"internalType\":\"structNodeStaking.StakingInfo\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"setAdminAddress\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"delay\",\"type\":\"uint256\"}],\"name\":\"setForceUnstakeDelay\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"stakeAmount\",\"type\":\"uint256\"}],\"name\":\"setMinStakeAmount\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"setObserver\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"slashReceiver\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"slashStaking\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"stakedAmount\",\"type\":\"uint256\"}],\"name\":\"stake\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"tryUnstake\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"unstake\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// NodeStakingABI is the input ABI used to generate the binding from.
// Deprecated: Use NodeStakingMetaData.ABI instead.
var NodeStakingABI = NodeStakingMetaData.ABI

// NodeStaking is an auto generated Go binding around an Ethereum contract.
type NodeStaking struct {
	NodeStakingCaller     // Read-only binding to the contract
	NodeStakingTransactor // Write-only binding to the contract
	NodeStakingFilterer   // Log filterer for contract events
}

// NodeStakingCaller is an auto generated read-only Go binding around an Ethereum contract.
type NodeStakingCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NodeStakingTransactor is an auto generated write-only Go binding around an Ethereum contract.
type NodeStakingTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NodeStakingFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type NodeStakingFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NodeStakingSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type NodeStakingSession struct {
	Contract     *NodeStaking      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// NodeStakingCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type NodeStakingCallerSession struct {
	Contract *NodeStakingCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// NodeStakingTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type NodeStakingTransactorSession struct {
	Contract     *NodeStakingTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// NodeStakingRaw is an auto generated low-level Go binding around an Ethereum contract.
type NodeStakingRaw struct {
	Contract *NodeStaking // Generic contract binding to access the raw methods on
}

// NodeStakingCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type NodeStakingCallerRaw struct {
	Contract *NodeStakingCaller // Generic read-only contract binding to access the raw methods on
}

// NodeStakingTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type NodeStakingTransactorRaw struct {
	Contract *NodeStakingTransactor // Generic write-only contract binding to access the raw methods on
}

// NewNodeStaking creates a new instance of NodeStaking, bound to a specific deployed contract.
func NewNodeStaking(address common.Address, backend bind.ContractBackend) (*NodeStaking, error) {
	contract, err := bindNodeStaking(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &NodeStaking{NodeStakingCaller: NodeStakingCaller{contract: contract}, NodeStakingTransactor: NodeStakingTransactor{contract: contract}, NodeStakingFilterer: NodeStakingFilterer{contract: contract}}, nil
}

// NewNodeStakingCaller creates a new read-only instance of NodeStaking, bound to a specific deployed contract.
func NewNodeStakingCaller(address common.Address, caller bind.ContractCaller) (*NodeStakingCaller, error) {
	contract, err := bindNodeStaking(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &NodeStakingCaller{contract: contract}, nil
}

// NewNodeStakingTransactor creates a new write-only instance of NodeStaking, bound to a specific deployed contract.
func NewNodeStakingTransactor(address common.Address, transactor bind.ContractTransactor) (*NodeStakingTransactor, error) {
	contract, err := bindNodeStaking(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &NodeStakingTransactor{contract: contract}, nil
}

// NewNodeStakingFilterer creates a new log filterer instance of NodeStaking, bound to a specific deployed contract.
func NewNodeStakingFilterer(address common.Address, filterer bind.ContractFilterer) (*NodeStakingFilterer, error) {
	contract, err := bindNodeStaking(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &NodeStakingFilterer{contract: contract}, nil
}

// bindNodeStaking binds a generic wrapper to an already deployed contract.
func bindNodeStaking(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := NodeStakingMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NodeStaking *NodeStakingRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NodeStaking.Contract.NodeStakingCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NodeStaking *NodeStakingRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NodeStaking.Contract.NodeStakingTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NodeStaking *NodeStakingRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NodeStaking.Contract.NodeStakingTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NodeStaking *NodeStakingCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NodeStaking.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NodeStaking *NodeStakingTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NodeStaking.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NodeStaking *NodeStakingTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NodeStaking.Contract.contract.Transact(opts, method, params...)
}

// Ba is a free data retrieval call binding the contract method 0x772604c1.
//
// Solidity: function ba() view returns(address)
func (_NodeStaking *NodeStakingCaller) Ba(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _NodeStaking.contract.Call(opts, &out, "ba")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Ba is a free data retrieval call binding the contract method 0x772604c1.
//
// Solidity: function ba() view returns(address)
func (_NodeStaking *NodeStakingSession) Ba() (common.Address, error) {
	return _NodeStaking.Contract.Ba(&_NodeStaking.CallOpts)
}

// Ba is a free data retrieval call binding the contract method 0x772604c1.
//
// Solidity: function ba() view returns(address)
func (_NodeStaking *NodeStakingCallerSession) Ba() (common.Address, error) {
	return _NodeStaking.Contract.Ba(&_NodeStaking.CallOpts)
}

// GetAllNodeAddresses is a free data retrieval call binding the contract method 0x6ba8ac44.
//
// Solidity: function getAllNodeAddresses(uint256 page, uint256 pageSize) view returns(address[])
func (_NodeStaking *NodeStakingCaller) GetAllNodeAddresses(opts *bind.CallOpts, page *big.Int, pageSize *big.Int) ([]common.Address, error) {
	var out []interface{}
	err := _NodeStaking.contract.Call(opts, &out, "getAllNodeAddresses", page, pageSize)

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetAllNodeAddresses is a free data retrieval call binding the contract method 0x6ba8ac44.
//
// Solidity: function getAllNodeAddresses(uint256 page, uint256 pageSize) view returns(address[])
func (_NodeStaking *NodeStakingSession) GetAllNodeAddresses(page *big.Int, pageSize *big.Int) ([]common.Address, error) {
	return _NodeStaking.Contract.GetAllNodeAddresses(&_NodeStaking.CallOpts, page, pageSize)
}

// GetAllNodeAddresses is a free data retrieval call binding the contract method 0x6ba8ac44.
//
// Solidity: function getAllNodeAddresses(uint256 page, uint256 pageSize) view returns(address[])
func (_NodeStaking *NodeStakingCallerSession) GetAllNodeAddresses(page *big.Int, pageSize *big.Int) ([]common.Address, error) {
	return _NodeStaking.Contract.GetAllNodeAddresses(&_NodeStaking.CallOpts, page, pageSize)
}

// GetForceUnstakeDelay is a free data retrieval call binding the contract method 0x1be835e2.
//
// Solidity: function getForceUnstakeDelay() view returns(uint256)
func (_NodeStaking *NodeStakingCaller) GetForceUnstakeDelay(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _NodeStaking.contract.Call(opts, &out, "getForceUnstakeDelay")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetForceUnstakeDelay is a free data retrieval call binding the contract method 0x1be835e2.
//
// Solidity: function getForceUnstakeDelay() view returns(uint256)
func (_NodeStaking *NodeStakingSession) GetForceUnstakeDelay() (*big.Int, error) {
	return _NodeStaking.Contract.GetForceUnstakeDelay(&_NodeStaking.CallOpts)
}

// GetForceUnstakeDelay is a free data retrieval call binding the contract method 0x1be835e2.
//
// Solidity: function getForceUnstakeDelay() view returns(uint256)
func (_NodeStaking *NodeStakingCallerSession) GetForceUnstakeDelay() (*big.Int, error) {
	return _NodeStaking.Contract.GetForceUnstakeDelay(&_NodeStaking.CallOpts)
}

// GetMinStakeAmount is a free data retrieval call binding the contract method 0x527cb1d7.
//
// Solidity: function getMinStakeAmount() view returns(uint256)
func (_NodeStaking *NodeStakingCaller) GetMinStakeAmount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _NodeStaking.contract.Call(opts, &out, "getMinStakeAmount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetMinStakeAmount is a free data retrieval call binding the contract method 0x527cb1d7.
//
// Solidity: function getMinStakeAmount() view returns(uint256)
func (_NodeStaking *NodeStakingSession) GetMinStakeAmount() (*big.Int, error) {
	return _NodeStaking.Contract.GetMinStakeAmount(&_NodeStaking.CallOpts)
}

// GetMinStakeAmount is a free data retrieval call binding the contract method 0x527cb1d7.
//
// Solidity: function getMinStakeAmount() view returns(uint256)
func (_NodeStaking *NodeStakingCallerSession) GetMinStakeAmount() (*big.Int, error) {
	return _NodeStaking.Contract.GetMinStakeAmount(&_NodeStaking.CallOpts)
}

// GetStakingInfo is a free data retrieval call binding the contract method 0xaa4704f3.
//
// Solidity: function getStakingInfo(address nodeAddress) view returns((address,uint256,uint8,uint256))
func (_NodeStaking *NodeStakingCaller) GetStakingInfo(opts *bind.CallOpts, nodeAddress common.Address) (NodeStakingStakingInfo, error) {
	var out []interface{}
	err := _NodeStaking.contract.Call(opts, &out, "getStakingInfo", nodeAddress)

	if err != nil {
		return *new(NodeStakingStakingInfo), err
	}

	out0 := *abi.ConvertType(out[0], new(NodeStakingStakingInfo)).(*NodeStakingStakingInfo)

	return out0, err

}

// GetStakingInfo is a free data retrieval call binding the contract method 0xaa4704f3.
//
// Solidity: function getStakingInfo(address nodeAddress) view returns((address,uint256,uint8,uint256))
func (_NodeStaking *NodeStakingSession) GetStakingInfo(nodeAddress common.Address) (NodeStakingStakingInfo, error) {
	return _NodeStaking.Contract.GetStakingInfo(&_NodeStaking.CallOpts, nodeAddress)
}

// GetStakingInfo is a free data retrieval call binding the contract method 0xaa4704f3.
//
// Solidity: function getStakingInfo(address nodeAddress) view returns((address,uint256,uint8,uint256))
func (_NodeStaking *NodeStakingCallerSession) GetStakingInfo(nodeAddress common.Address) (NodeStakingStakingInfo, error) {
	return _NodeStaking.Contract.GetStakingInfo(&_NodeStaking.CallOpts, nodeAddress)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_NodeStaking *NodeStakingCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _NodeStaking.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_NodeStaking *NodeStakingSession) Owner() (common.Address, error) {
	return _NodeStaking.Contract.Owner(&_NodeStaking.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_NodeStaking *NodeStakingCallerSession) Owner() (common.Address, error) {
	return _NodeStaking.Contract.Owner(&_NodeStaking.CallOpts)
}

// RenounceOwnership is a free data retrieval call binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() view returns()
func (_NodeStaking *NodeStakingCaller) RenounceOwnership(opts *bind.CallOpts) error {
	var out []interface{}
	err := _NodeStaking.contract.Call(opts, &out, "renounceOwnership")

	if err != nil {
		return err
	}

	return err

}

// RenounceOwnership is a free data retrieval call binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() view returns()
func (_NodeStaking *NodeStakingSession) RenounceOwnership() error {
	return _NodeStaking.Contract.RenounceOwnership(&_NodeStaking.CallOpts)
}

// RenounceOwnership is a free data retrieval call binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() view returns()
func (_NodeStaking *NodeStakingCallerSession) RenounceOwnership() error {
	return _NodeStaking.Contract.RenounceOwnership(&_NodeStaking.CallOpts)
}

// SlashReceiver is a free data retrieval call binding the contract method 0x1bc4e5fb.
//
// Solidity: function slashReceiver() view returns(address)
func (_NodeStaking *NodeStakingCaller) SlashReceiver(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _NodeStaking.contract.Call(opts, &out, "slashReceiver")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// SlashReceiver is a free data retrieval call binding the contract method 0x1bc4e5fb.
//
// Solidity: function slashReceiver() view returns(address)
func (_NodeStaking *NodeStakingSession) SlashReceiver() (common.Address, error) {
	return _NodeStaking.Contract.SlashReceiver(&_NodeStaking.CallOpts)
}

// SlashReceiver is a free data retrieval call binding the contract method 0x1bc4e5fb.
//
// Solidity: function slashReceiver() view returns(address)
func (_NodeStaking *NodeStakingCallerSession) SlashReceiver() (common.Address, error) {
	return _NodeStaking.Contract.SlashReceiver(&_NodeStaking.CallOpts)
}

// ForceUnstake is a paid mutator transaction binding the contract method 0xdf4bbd22.
//
// Solidity: function forceUnstake() returns()
func (_NodeStaking *NodeStakingTransactor) ForceUnstake(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NodeStaking.contract.Transact(opts, "forceUnstake")
}

// ForceUnstake is a paid mutator transaction binding the contract method 0xdf4bbd22.
//
// Solidity: function forceUnstake() returns()
func (_NodeStaking *NodeStakingSession) ForceUnstake() (*types.Transaction, error) {
	return _NodeStaking.Contract.ForceUnstake(&_NodeStaking.TransactOpts)
}

// ForceUnstake is a paid mutator transaction binding the contract method 0xdf4bbd22.
//
// Solidity: function forceUnstake() returns()
func (_NodeStaking *NodeStakingTransactorSession) ForceUnstake() (*types.Transaction, error) {
	return _NodeStaking.Contract.ForceUnstake(&_NodeStaking.TransactOpts)
}

// SetAdminAddress is a paid mutator transaction binding the contract method 0x2c1e816d.
//
// Solidity: function setAdminAddress(address addr) returns()
func (_NodeStaking *NodeStakingTransactor) SetAdminAddress(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _NodeStaking.contract.Transact(opts, "setAdminAddress", addr)
}

// SetAdminAddress is a paid mutator transaction binding the contract method 0x2c1e816d.
//
// Solidity: function setAdminAddress(address addr) returns()
func (_NodeStaking *NodeStakingSession) SetAdminAddress(addr common.Address) (*types.Transaction, error) {
	return _NodeStaking.Contract.SetAdminAddress(&_NodeStaking.TransactOpts, addr)
}

// SetAdminAddress is a paid mutator transaction binding the contract method 0x2c1e816d.
//
// Solidity: function setAdminAddress(address addr) returns()
func (_NodeStaking *NodeStakingTransactorSession) SetAdminAddress(addr common.Address) (*types.Transaction, error) {
	return _NodeStaking.Contract.SetAdminAddress(&_NodeStaking.TransactOpts, addr)
}

// SetForceUnstakeDelay is a paid mutator transaction binding the contract method 0x560dc313.
//
// Solidity: function setForceUnstakeDelay(uint256 delay) returns()
func (_NodeStaking *NodeStakingTransactor) SetForceUnstakeDelay(opts *bind.TransactOpts, delay *big.Int) (*types.Transaction, error) {
	return _NodeStaking.contract.Transact(opts, "setForceUnstakeDelay", delay)
}

// SetForceUnstakeDelay is a paid mutator transaction binding the contract method 0x560dc313.
//
// Solidity: function setForceUnstakeDelay(uint256 delay) returns()
func (_NodeStaking *NodeStakingSession) SetForceUnstakeDelay(delay *big.Int) (*types.Transaction, error) {
	return _NodeStaking.Contract.SetForceUnstakeDelay(&_NodeStaking.TransactOpts, delay)
}

// SetForceUnstakeDelay is a paid mutator transaction binding the contract method 0x560dc313.
//
// Solidity: function setForceUnstakeDelay(uint256 delay) returns()
func (_NodeStaking *NodeStakingTransactorSession) SetForceUnstakeDelay(delay *big.Int) (*types.Transaction, error) {
	return _NodeStaking.Contract.SetForceUnstakeDelay(&_NodeStaking.TransactOpts, delay)
}

// SetMinStakeAmount is a paid mutator transaction binding the contract method 0xeb4af045.
//
// Solidity: function setMinStakeAmount(uint256 stakeAmount) returns()
func (_NodeStaking *NodeStakingTransactor) SetMinStakeAmount(opts *bind.TransactOpts, stakeAmount *big.Int) (*types.Transaction, error) {
	return _NodeStaking.contract.Transact(opts, "setMinStakeAmount", stakeAmount)
}

// SetMinStakeAmount is a paid mutator transaction binding the contract method 0xeb4af045.
//
// Solidity: function setMinStakeAmount(uint256 stakeAmount) returns()
func (_NodeStaking *NodeStakingSession) SetMinStakeAmount(stakeAmount *big.Int) (*types.Transaction, error) {
	return _NodeStaking.Contract.SetMinStakeAmount(&_NodeStaking.TransactOpts, stakeAmount)
}

// SetMinStakeAmount is a paid mutator transaction binding the contract method 0xeb4af045.
//
// Solidity: function setMinStakeAmount(uint256 stakeAmount) returns()
func (_NodeStaking *NodeStakingTransactorSession) SetMinStakeAmount(stakeAmount *big.Int) (*types.Transaction, error) {
	return _NodeStaking.Contract.SetMinStakeAmount(&_NodeStaking.TransactOpts, stakeAmount)
}

// SetObserver is a paid mutator transaction binding the contract method 0x94d9c9c7.
//
// Solidity: function setObserver(address addr) returns()
func (_NodeStaking *NodeStakingTransactor) SetObserver(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _NodeStaking.contract.Transact(opts, "setObserver", addr)
}

// SetObserver is a paid mutator transaction binding the contract method 0x94d9c9c7.
//
// Solidity: function setObserver(address addr) returns()
func (_NodeStaking *NodeStakingSession) SetObserver(addr common.Address) (*types.Transaction, error) {
	return _NodeStaking.Contract.SetObserver(&_NodeStaking.TransactOpts, addr)
}

// SetObserver is a paid mutator transaction binding the contract method 0x94d9c9c7.
//
// Solidity: function setObserver(address addr) returns()
func (_NodeStaking *NodeStakingTransactorSession) SetObserver(addr common.Address) (*types.Transaction, error) {
	return _NodeStaking.Contract.SetObserver(&_NodeStaking.TransactOpts, addr)
}

// SlashStaking is a paid mutator transaction binding the contract method 0xf7999cb1.
//
// Solidity: function slashStaking(address nodeAddress) returns()
func (_NodeStaking *NodeStakingTransactor) SlashStaking(opts *bind.TransactOpts, nodeAddress common.Address) (*types.Transaction, error) {
	return _NodeStaking.contract.Transact(opts, "slashStaking", nodeAddress)
}

// SlashStaking is a paid mutator transaction binding the contract method 0xf7999cb1.
//
// Solidity: function slashStaking(address nodeAddress) returns()
func (_NodeStaking *NodeStakingSession) SlashStaking(nodeAddress common.Address) (*types.Transaction, error) {
	return _NodeStaking.Contract.SlashStaking(&_NodeStaking.TransactOpts, nodeAddress)
}

// SlashStaking is a paid mutator transaction binding the contract method 0xf7999cb1.
//
// Solidity: function slashStaking(address nodeAddress) returns()
func (_NodeStaking *NodeStakingTransactorSession) SlashStaking(nodeAddress common.Address) (*types.Transaction, error) {
	return _NodeStaking.Contract.SlashStaking(&_NodeStaking.TransactOpts, nodeAddress)
}

// Stake is a paid mutator transaction binding the contract method 0xa694fc3a.
//
// Solidity: function stake(uint256 stakedAmount) payable returns()
func (_NodeStaking *NodeStakingTransactor) Stake(opts *bind.TransactOpts, stakedAmount *big.Int) (*types.Transaction, error) {
	return _NodeStaking.contract.Transact(opts, "stake", stakedAmount)
}

// Stake is a paid mutator transaction binding the contract method 0xa694fc3a.
//
// Solidity: function stake(uint256 stakedAmount) payable returns()
func (_NodeStaking *NodeStakingSession) Stake(stakedAmount *big.Int) (*types.Transaction, error) {
	return _NodeStaking.Contract.Stake(&_NodeStaking.TransactOpts, stakedAmount)
}

// Stake is a paid mutator transaction binding the contract method 0xa694fc3a.
//
// Solidity: function stake(uint256 stakedAmount) payable returns()
func (_NodeStaking *NodeStakingTransactorSession) Stake(stakedAmount *big.Int) (*types.Transaction, error) {
	return _NodeStaking.Contract.Stake(&_NodeStaking.TransactOpts, stakedAmount)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_NodeStaking *NodeStakingTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _NodeStaking.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_NodeStaking *NodeStakingSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _NodeStaking.Contract.TransferOwnership(&_NodeStaking.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_NodeStaking *NodeStakingTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _NodeStaking.Contract.TransferOwnership(&_NodeStaking.TransactOpts, newOwner)
}

// TryUnstake is a paid mutator transaction binding the contract method 0x91a018ce.
//
// Solidity: function tryUnstake() returns()
func (_NodeStaking *NodeStakingTransactor) TryUnstake(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NodeStaking.contract.Transact(opts, "tryUnstake")
}

// TryUnstake is a paid mutator transaction binding the contract method 0x91a018ce.
//
// Solidity: function tryUnstake() returns()
func (_NodeStaking *NodeStakingSession) TryUnstake() (*types.Transaction, error) {
	return _NodeStaking.Contract.TryUnstake(&_NodeStaking.TransactOpts)
}

// TryUnstake is a paid mutator transaction binding the contract method 0x91a018ce.
//
// Solidity: function tryUnstake() returns()
func (_NodeStaking *NodeStakingTransactorSession) TryUnstake() (*types.Transaction, error) {
	return _NodeStaking.Contract.TryUnstake(&_NodeStaking.TransactOpts)
}

// Unstake is a paid mutator transaction binding the contract method 0xf2888dbb.
//
// Solidity: function unstake(address nodeAddress) returns()
func (_NodeStaking *NodeStakingTransactor) Unstake(opts *bind.TransactOpts, nodeAddress common.Address) (*types.Transaction, error) {
	return _NodeStaking.contract.Transact(opts, "unstake", nodeAddress)
}

// Unstake is a paid mutator transaction binding the contract method 0xf2888dbb.
//
// Solidity: function unstake(address nodeAddress) returns()
func (_NodeStaking *NodeStakingSession) Unstake(nodeAddress common.Address) (*types.Transaction, error) {
	return _NodeStaking.Contract.Unstake(&_NodeStaking.TransactOpts, nodeAddress)
}

// Unstake is a paid mutator transaction binding the contract method 0xf2888dbb.
//
// Solidity: function unstake(address nodeAddress) returns()
func (_NodeStaking *NodeStakingTransactorSession) Unstake(nodeAddress common.Address) (*types.Transaction, error) {
	return _NodeStaking.Contract.Unstake(&_NodeStaking.TransactOpts, nodeAddress)
}

// NodeStakingAdminAddressUpdatedIterator is returned from FilterAdminAddressUpdated and is used to iterate over the raw logs and unpacked data for AdminAddressUpdated events raised by the NodeStaking contract.
type NodeStakingAdminAddressUpdatedIterator struct {
	Event *NodeStakingAdminAddressUpdated // Event containing the contract specifics and raw log

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
func (it *NodeStakingAdminAddressUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodeStakingAdminAddressUpdated)
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
		it.Event = new(NodeStakingAdminAddressUpdated)
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
func (it *NodeStakingAdminAddressUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodeStakingAdminAddressUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodeStakingAdminAddressUpdated represents a AdminAddressUpdated event raised by the NodeStaking contract.
type NodeStakingAdminAddressUpdated struct {
	OldAddress common.Address
	NewAddress common.Address
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterAdminAddressUpdated is a free log retrieval operation binding the contract event 0x39eb67354e1d981c44468f6a2b1837bb1598cf711fe761db800b185706b4e8cb.
//
// Solidity: event AdminAddressUpdated(address indexed oldAddress, address indexed newAddress)
func (_NodeStaking *NodeStakingFilterer) FilterAdminAddressUpdated(opts *bind.FilterOpts, oldAddress []common.Address, newAddress []common.Address) (*NodeStakingAdminAddressUpdatedIterator, error) {

	var oldAddressRule []interface{}
	for _, oldAddressItem := range oldAddress {
		oldAddressRule = append(oldAddressRule, oldAddressItem)
	}
	var newAddressRule []interface{}
	for _, newAddressItem := range newAddress {
		newAddressRule = append(newAddressRule, newAddressItem)
	}

	logs, sub, err := _NodeStaking.contract.FilterLogs(opts, "AdminAddressUpdated", oldAddressRule, newAddressRule)
	if err != nil {
		return nil, err
	}
	return &NodeStakingAdminAddressUpdatedIterator{contract: _NodeStaking.contract, event: "AdminAddressUpdated", logs: logs, sub: sub}, nil
}

// WatchAdminAddressUpdated is a free log subscription operation binding the contract event 0x39eb67354e1d981c44468f6a2b1837bb1598cf711fe761db800b185706b4e8cb.
//
// Solidity: event AdminAddressUpdated(address indexed oldAddress, address indexed newAddress)
func (_NodeStaking *NodeStakingFilterer) WatchAdminAddressUpdated(opts *bind.WatchOpts, sink chan<- *NodeStakingAdminAddressUpdated, oldAddress []common.Address, newAddress []common.Address) (event.Subscription, error) {

	var oldAddressRule []interface{}
	for _, oldAddressItem := range oldAddress {
		oldAddressRule = append(oldAddressRule, oldAddressItem)
	}
	var newAddressRule []interface{}
	for _, newAddressItem := range newAddress {
		newAddressRule = append(newAddressRule, newAddressItem)
	}

	logs, sub, err := _NodeStaking.contract.WatchLogs(opts, "AdminAddressUpdated", oldAddressRule, newAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodeStakingAdminAddressUpdated)
				if err := _NodeStaking.contract.UnpackLog(event, "AdminAddressUpdated", log); err != nil {
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

// ParseAdminAddressUpdated is a log parse operation binding the contract event 0x39eb67354e1d981c44468f6a2b1837bb1598cf711fe761db800b185706b4e8cb.
//
// Solidity: event AdminAddressUpdated(address indexed oldAddress, address indexed newAddress)
func (_NodeStaking *NodeStakingFilterer) ParseAdminAddressUpdated(log types.Log) (*NodeStakingAdminAddressUpdated, error) {
	event := new(NodeStakingAdminAddressUpdated)
	if err := _NodeStaking.contract.UnpackLog(event, "AdminAddressUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NodeStakingForceUnstakeDelayUpdatedIterator is returned from FilterForceUnstakeDelayUpdated and is used to iterate over the raw logs and unpacked data for ForceUnstakeDelayUpdated events raised by the NodeStaking contract.
type NodeStakingForceUnstakeDelayUpdatedIterator struct {
	Event *NodeStakingForceUnstakeDelayUpdated // Event containing the contract specifics and raw log

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
func (it *NodeStakingForceUnstakeDelayUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodeStakingForceUnstakeDelayUpdated)
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
		it.Event = new(NodeStakingForceUnstakeDelayUpdated)
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
func (it *NodeStakingForceUnstakeDelayUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodeStakingForceUnstakeDelayUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodeStakingForceUnstakeDelayUpdated represents a ForceUnstakeDelayUpdated event raised by the NodeStaking contract.
type NodeStakingForceUnstakeDelayUpdated struct {
	OldDelay *big.Int
	NewDelay *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterForceUnstakeDelayUpdated is a free log retrieval operation binding the contract event 0x2324fb4874e07c77ab5d8b58eb3aa9da7d4027ea88d8d08142dd0963b7ef7351.
//
// Solidity: event ForceUnstakeDelayUpdated(uint256 oldDelay, uint256 newDelay)
func (_NodeStaking *NodeStakingFilterer) FilterForceUnstakeDelayUpdated(opts *bind.FilterOpts) (*NodeStakingForceUnstakeDelayUpdatedIterator, error) {

	logs, sub, err := _NodeStaking.contract.FilterLogs(opts, "ForceUnstakeDelayUpdated")
	if err != nil {
		return nil, err
	}
	return &NodeStakingForceUnstakeDelayUpdatedIterator{contract: _NodeStaking.contract, event: "ForceUnstakeDelayUpdated", logs: logs, sub: sub}, nil
}

// WatchForceUnstakeDelayUpdated is a free log subscription operation binding the contract event 0x2324fb4874e07c77ab5d8b58eb3aa9da7d4027ea88d8d08142dd0963b7ef7351.
//
// Solidity: event ForceUnstakeDelayUpdated(uint256 oldDelay, uint256 newDelay)
func (_NodeStaking *NodeStakingFilterer) WatchForceUnstakeDelayUpdated(opts *bind.WatchOpts, sink chan<- *NodeStakingForceUnstakeDelayUpdated) (event.Subscription, error) {

	logs, sub, err := _NodeStaking.contract.WatchLogs(opts, "ForceUnstakeDelayUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodeStakingForceUnstakeDelayUpdated)
				if err := _NodeStaking.contract.UnpackLog(event, "ForceUnstakeDelayUpdated", log); err != nil {
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

// ParseForceUnstakeDelayUpdated is a log parse operation binding the contract event 0x2324fb4874e07c77ab5d8b58eb3aa9da7d4027ea88d8d08142dd0963b7ef7351.
//
// Solidity: event ForceUnstakeDelayUpdated(uint256 oldDelay, uint256 newDelay)
func (_NodeStaking *NodeStakingFilterer) ParseForceUnstakeDelayUpdated(log types.Log) (*NodeStakingForceUnstakeDelayUpdated, error) {
	event := new(NodeStakingForceUnstakeDelayUpdated)
	if err := _NodeStaking.contract.UnpackLog(event, "ForceUnstakeDelayUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NodeStakingMinStakeAmountUpdatedIterator is returned from FilterMinStakeAmountUpdated and is used to iterate over the raw logs and unpacked data for MinStakeAmountUpdated events raised by the NodeStaking contract.
type NodeStakingMinStakeAmountUpdatedIterator struct {
	Event *NodeStakingMinStakeAmountUpdated // Event containing the contract specifics and raw log

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
func (it *NodeStakingMinStakeAmountUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodeStakingMinStakeAmountUpdated)
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
		it.Event = new(NodeStakingMinStakeAmountUpdated)
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
func (it *NodeStakingMinStakeAmountUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodeStakingMinStakeAmountUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodeStakingMinStakeAmountUpdated represents a MinStakeAmountUpdated event raised by the NodeStaking contract.
type NodeStakingMinStakeAmountUpdated struct {
	OldAmount *big.Int
	NewAmount *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterMinStakeAmountUpdated is a free log retrieval operation binding the contract event 0xca0542093af2ac14ccf6e52b6e1a131c7e2825fb3b51139bf1dd8186a1339e95.
//
// Solidity: event MinStakeAmountUpdated(uint256 oldAmount, uint256 newAmount)
func (_NodeStaking *NodeStakingFilterer) FilterMinStakeAmountUpdated(opts *bind.FilterOpts) (*NodeStakingMinStakeAmountUpdatedIterator, error) {

	logs, sub, err := _NodeStaking.contract.FilterLogs(opts, "MinStakeAmountUpdated")
	if err != nil {
		return nil, err
	}
	return &NodeStakingMinStakeAmountUpdatedIterator{contract: _NodeStaking.contract, event: "MinStakeAmountUpdated", logs: logs, sub: sub}, nil
}

// WatchMinStakeAmountUpdated is a free log subscription operation binding the contract event 0xca0542093af2ac14ccf6e52b6e1a131c7e2825fb3b51139bf1dd8186a1339e95.
//
// Solidity: event MinStakeAmountUpdated(uint256 oldAmount, uint256 newAmount)
func (_NodeStaking *NodeStakingFilterer) WatchMinStakeAmountUpdated(opts *bind.WatchOpts, sink chan<- *NodeStakingMinStakeAmountUpdated) (event.Subscription, error) {

	logs, sub, err := _NodeStaking.contract.WatchLogs(opts, "MinStakeAmountUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodeStakingMinStakeAmountUpdated)
				if err := _NodeStaking.contract.UnpackLog(event, "MinStakeAmountUpdated", log); err != nil {
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

// ParseMinStakeAmountUpdated is a log parse operation binding the contract event 0xca0542093af2ac14ccf6e52b6e1a131c7e2825fb3b51139bf1dd8186a1339e95.
//
// Solidity: event MinStakeAmountUpdated(uint256 oldAmount, uint256 newAmount)
func (_NodeStaking *NodeStakingFilterer) ParseMinStakeAmountUpdated(log types.Log) (*NodeStakingMinStakeAmountUpdated, error) {
	event := new(NodeStakingMinStakeAmountUpdated)
	if err := _NodeStaking.contract.UnpackLog(event, "MinStakeAmountUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NodeStakingNodeSlashedIterator is returned from FilterNodeSlashed and is used to iterate over the raw logs and unpacked data for NodeSlashed events raised by the NodeStaking contract.
type NodeStakingNodeSlashedIterator struct {
	Event *NodeStakingNodeSlashed // Event containing the contract specifics and raw log

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
func (it *NodeStakingNodeSlashedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodeStakingNodeSlashed)
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
		it.Event = new(NodeStakingNodeSlashed)
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
func (it *NodeStakingNodeSlashedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodeStakingNodeSlashedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodeStakingNodeSlashed represents a NodeSlashed event raised by the NodeStaking contract.
type NodeStakingNodeSlashed struct {
	NodeAddress   common.Address
	StakedBalance *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterNodeSlashed is a free log retrieval operation binding the contract event 0x51cf713376ddb1e5f5828bb6aa39d99de812176d62c3d3550bdc4e0b5e86e1a5.
//
// Solidity: event NodeSlashed(address indexed nodeAddress, uint256 stakedBalance)
func (_NodeStaking *NodeStakingFilterer) FilterNodeSlashed(opts *bind.FilterOpts, nodeAddress []common.Address) (*NodeStakingNodeSlashedIterator, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _NodeStaking.contract.FilterLogs(opts, "NodeSlashed", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return &NodeStakingNodeSlashedIterator{contract: _NodeStaking.contract, event: "NodeSlashed", logs: logs, sub: sub}, nil
}

// WatchNodeSlashed is a free log subscription operation binding the contract event 0x51cf713376ddb1e5f5828bb6aa39d99de812176d62c3d3550bdc4e0b5e86e1a5.
//
// Solidity: event NodeSlashed(address indexed nodeAddress, uint256 stakedBalance)
func (_NodeStaking *NodeStakingFilterer) WatchNodeSlashed(opts *bind.WatchOpts, sink chan<- *NodeStakingNodeSlashed, nodeAddress []common.Address) (event.Subscription, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _NodeStaking.contract.WatchLogs(opts, "NodeSlashed", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodeStakingNodeSlashed)
				if err := _NodeStaking.contract.UnpackLog(event, "NodeSlashed", log); err != nil {
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

// ParseNodeSlashed is a log parse operation binding the contract event 0x51cf713376ddb1e5f5828bb6aa39d99de812176d62c3d3550bdc4e0b5e86e1a5.
//
// Solidity: event NodeSlashed(address indexed nodeAddress, uint256 stakedBalance)
func (_NodeStaking *NodeStakingFilterer) ParseNodeSlashed(log types.Log) (*NodeStakingNodeSlashed, error) {
	event := new(NodeStakingNodeSlashed)
	if err := _NodeStaking.contract.UnpackLog(event, "NodeSlashed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NodeStakingNodeStakedIterator is returned from FilterNodeStaked and is used to iterate over the raw logs and unpacked data for NodeStaked events raised by the NodeStaking contract.
type NodeStakingNodeStakedIterator struct {
	Event *NodeStakingNodeStaked // Event containing the contract specifics and raw log

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
func (it *NodeStakingNodeStakedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodeStakingNodeStaked)
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
		it.Event = new(NodeStakingNodeStaked)
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
func (it *NodeStakingNodeStakedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodeStakingNodeStakedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodeStakingNodeStaked represents a NodeStaked event raised by the NodeStaking contract.
type NodeStakingNodeStaked struct {
	NodeAddress   common.Address
	StakedBalance *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterNodeStaked is a free log retrieval operation binding the contract event 0xf02b9c90d1a12278c9244be1ddc794b15f8a3b9d50cf085ad7711683c0f9a090.
//
// Solidity: event NodeStaked(address indexed nodeAddress, uint256 stakedBalance)
func (_NodeStaking *NodeStakingFilterer) FilterNodeStaked(opts *bind.FilterOpts, nodeAddress []common.Address) (*NodeStakingNodeStakedIterator, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _NodeStaking.contract.FilterLogs(opts, "NodeStaked", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return &NodeStakingNodeStakedIterator{contract: _NodeStaking.contract, event: "NodeStaked", logs: logs, sub: sub}, nil
}

// WatchNodeStaked is a free log subscription operation binding the contract event 0xf02b9c90d1a12278c9244be1ddc794b15f8a3b9d50cf085ad7711683c0f9a090.
//
// Solidity: event NodeStaked(address indexed nodeAddress, uint256 stakedBalance)
func (_NodeStaking *NodeStakingFilterer) WatchNodeStaked(opts *bind.WatchOpts, sink chan<- *NodeStakingNodeStaked, nodeAddress []common.Address) (event.Subscription, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _NodeStaking.contract.WatchLogs(opts, "NodeStaked", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodeStakingNodeStaked)
				if err := _NodeStaking.contract.UnpackLog(event, "NodeStaked", log); err != nil {
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

// ParseNodeStaked is a log parse operation binding the contract event 0xf02b9c90d1a12278c9244be1ddc794b15f8a3b9d50cf085ad7711683c0f9a090.
//
// Solidity: event NodeStaked(address indexed nodeAddress, uint256 stakedBalance)
func (_NodeStaking *NodeStakingFilterer) ParseNodeStaked(log types.Log) (*NodeStakingNodeStaked, error) {
	event := new(NodeStakingNodeStaked)
	if err := _NodeStaking.contract.UnpackLog(event, "NodeStaked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NodeStakingNodeTryUnstakedIterator is returned from FilterNodeTryUnstaked and is used to iterate over the raw logs and unpacked data for NodeTryUnstaked events raised by the NodeStaking contract.
type NodeStakingNodeTryUnstakedIterator struct {
	Event *NodeStakingNodeTryUnstaked // Event containing the contract specifics and raw log

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
func (it *NodeStakingNodeTryUnstakedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodeStakingNodeTryUnstaked)
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
		it.Event = new(NodeStakingNodeTryUnstaked)
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
func (it *NodeStakingNodeTryUnstakedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodeStakingNodeTryUnstakedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodeStakingNodeTryUnstaked represents a NodeTryUnstaked event raised by the NodeStaking contract.
type NodeStakingNodeTryUnstaked struct {
	NodeAddress common.Address
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterNodeTryUnstaked is a free log retrieval operation binding the contract event 0xdded5e49923cf415baa8ad1d77f75f5b8f5ecf78b7572a9eedd0a39994036d19.
//
// Solidity: event NodeTryUnstaked(address indexed nodeAddress)
func (_NodeStaking *NodeStakingFilterer) FilterNodeTryUnstaked(opts *bind.FilterOpts, nodeAddress []common.Address) (*NodeStakingNodeTryUnstakedIterator, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _NodeStaking.contract.FilterLogs(opts, "NodeTryUnstaked", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return &NodeStakingNodeTryUnstakedIterator{contract: _NodeStaking.contract, event: "NodeTryUnstaked", logs: logs, sub: sub}, nil
}

// WatchNodeTryUnstaked is a free log subscription operation binding the contract event 0xdded5e49923cf415baa8ad1d77f75f5b8f5ecf78b7572a9eedd0a39994036d19.
//
// Solidity: event NodeTryUnstaked(address indexed nodeAddress)
func (_NodeStaking *NodeStakingFilterer) WatchNodeTryUnstaked(opts *bind.WatchOpts, sink chan<- *NodeStakingNodeTryUnstaked, nodeAddress []common.Address) (event.Subscription, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _NodeStaking.contract.WatchLogs(opts, "NodeTryUnstaked", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodeStakingNodeTryUnstaked)
				if err := _NodeStaking.contract.UnpackLog(event, "NodeTryUnstaked", log); err != nil {
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

// ParseNodeTryUnstaked is a log parse operation binding the contract event 0xdded5e49923cf415baa8ad1d77f75f5b8f5ecf78b7572a9eedd0a39994036d19.
//
// Solidity: event NodeTryUnstaked(address indexed nodeAddress)
func (_NodeStaking *NodeStakingFilterer) ParseNodeTryUnstaked(log types.Log) (*NodeStakingNodeTryUnstaked, error) {
	event := new(NodeStakingNodeTryUnstaked)
	if err := _NodeStaking.contract.UnpackLog(event, "NodeTryUnstaked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NodeStakingNodeUnstakedIterator is returned from FilterNodeUnstaked and is used to iterate over the raw logs and unpacked data for NodeUnstaked events raised by the NodeStaking contract.
type NodeStakingNodeUnstakedIterator struct {
	Event *NodeStakingNodeUnstaked // Event containing the contract specifics and raw log

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
func (it *NodeStakingNodeUnstakedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodeStakingNodeUnstaked)
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
		it.Event = new(NodeStakingNodeUnstaked)
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
func (it *NodeStakingNodeUnstakedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodeStakingNodeUnstakedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodeStakingNodeUnstaked represents a NodeUnstaked event raised by the NodeStaking contract.
type NodeStakingNodeUnstaked struct {
	NodeAddress   common.Address
	StakedBalance *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterNodeUnstaked is a free log retrieval operation binding the contract event 0x67722cd735ac17faf7e2ff2d57ac864a5a1f41405ee9113c16a413344ce31fbf.
//
// Solidity: event NodeUnstaked(address indexed nodeAddress, uint256 stakedBalance)
func (_NodeStaking *NodeStakingFilterer) FilterNodeUnstaked(opts *bind.FilterOpts, nodeAddress []common.Address) (*NodeStakingNodeUnstakedIterator, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _NodeStaking.contract.FilterLogs(opts, "NodeUnstaked", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return &NodeStakingNodeUnstakedIterator{contract: _NodeStaking.contract, event: "NodeUnstaked", logs: logs, sub: sub}, nil
}

// WatchNodeUnstaked is a free log subscription operation binding the contract event 0x67722cd735ac17faf7e2ff2d57ac864a5a1f41405ee9113c16a413344ce31fbf.
//
// Solidity: event NodeUnstaked(address indexed nodeAddress, uint256 stakedBalance)
func (_NodeStaking *NodeStakingFilterer) WatchNodeUnstaked(opts *bind.WatchOpts, sink chan<- *NodeStakingNodeUnstaked, nodeAddress []common.Address) (event.Subscription, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _NodeStaking.contract.WatchLogs(opts, "NodeUnstaked", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodeStakingNodeUnstaked)
				if err := _NodeStaking.contract.UnpackLog(event, "NodeUnstaked", log); err != nil {
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

// ParseNodeUnstaked is a log parse operation binding the contract event 0x67722cd735ac17faf7e2ff2d57ac864a5a1f41405ee9113c16a413344ce31fbf.
//
// Solidity: event NodeUnstaked(address indexed nodeAddress, uint256 stakedBalance)
func (_NodeStaking *NodeStakingFilterer) ParseNodeUnstaked(log types.Log) (*NodeStakingNodeUnstaked, error) {
	event := new(NodeStakingNodeUnstaked)
	if err := _NodeStaking.contract.UnpackLog(event, "NodeUnstaked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NodeStakingObserverUpdatedIterator is returned from FilterObserverUpdated and is used to iterate over the raw logs and unpacked data for ObserverUpdated events raised by the NodeStaking contract.
type NodeStakingObserverUpdatedIterator struct {
	Event *NodeStakingObserverUpdated // Event containing the contract specifics and raw log

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
func (it *NodeStakingObserverUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodeStakingObserverUpdated)
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
		it.Event = new(NodeStakingObserverUpdated)
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
func (it *NodeStakingObserverUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodeStakingObserverUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodeStakingObserverUpdated represents a ObserverUpdated event raised by the NodeStaking contract.
type NodeStakingObserverUpdated struct {
	OldObserver common.Address
	NewObserver common.Address
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterObserverUpdated is a free log retrieval operation binding the contract event 0x5d62ce8ed7aebf9654df2f8c5372e47942b59bcb73a5d26ac71edaf9f670daec.
//
// Solidity: event ObserverUpdated(address indexed oldObserver, address indexed newObserver)
func (_NodeStaking *NodeStakingFilterer) FilterObserverUpdated(opts *bind.FilterOpts, oldObserver []common.Address, newObserver []common.Address) (*NodeStakingObserverUpdatedIterator, error) {

	var oldObserverRule []interface{}
	for _, oldObserverItem := range oldObserver {
		oldObserverRule = append(oldObserverRule, oldObserverItem)
	}
	var newObserverRule []interface{}
	for _, newObserverItem := range newObserver {
		newObserverRule = append(newObserverRule, newObserverItem)
	}

	logs, sub, err := _NodeStaking.contract.FilterLogs(opts, "ObserverUpdated", oldObserverRule, newObserverRule)
	if err != nil {
		return nil, err
	}
	return &NodeStakingObserverUpdatedIterator{contract: _NodeStaking.contract, event: "ObserverUpdated", logs: logs, sub: sub}, nil
}

// WatchObserverUpdated is a free log subscription operation binding the contract event 0x5d62ce8ed7aebf9654df2f8c5372e47942b59bcb73a5d26ac71edaf9f670daec.
//
// Solidity: event ObserverUpdated(address indexed oldObserver, address indexed newObserver)
func (_NodeStaking *NodeStakingFilterer) WatchObserverUpdated(opts *bind.WatchOpts, sink chan<- *NodeStakingObserverUpdated, oldObserver []common.Address, newObserver []common.Address) (event.Subscription, error) {

	var oldObserverRule []interface{}
	for _, oldObserverItem := range oldObserver {
		oldObserverRule = append(oldObserverRule, oldObserverItem)
	}
	var newObserverRule []interface{}
	for _, newObserverItem := range newObserver {
		newObserverRule = append(newObserverRule, newObserverItem)
	}

	logs, sub, err := _NodeStaking.contract.WatchLogs(opts, "ObserverUpdated", oldObserverRule, newObserverRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodeStakingObserverUpdated)
				if err := _NodeStaking.contract.UnpackLog(event, "ObserverUpdated", log); err != nil {
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

// ParseObserverUpdated is a log parse operation binding the contract event 0x5d62ce8ed7aebf9654df2f8c5372e47942b59bcb73a5d26ac71edaf9f670daec.
//
// Solidity: event ObserverUpdated(address indexed oldObserver, address indexed newObserver)
func (_NodeStaking *NodeStakingFilterer) ParseObserverUpdated(log types.Log) (*NodeStakingObserverUpdated, error) {
	event := new(NodeStakingObserverUpdated)
	if err := _NodeStaking.contract.UnpackLog(event, "ObserverUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NodeStakingOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the NodeStaking contract.
type NodeStakingOwnershipTransferredIterator struct {
	Event *NodeStakingOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *NodeStakingOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodeStakingOwnershipTransferred)
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
		it.Event = new(NodeStakingOwnershipTransferred)
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
func (it *NodeStakingOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodeStakingOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodeStakingOwnershipTransferred represents a OwnershipTransferred event raised by the NodeStaking contract.
type NodeStakingOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_NodeStaking *NodeStakingFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*NodeStakingOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _NodeStaking.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &NodeStakingOwnershipTransferredIterator{contract: _NodeStaking.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_NodeStaking *NodeStakingFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *NodeStakingOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _NodeStaking.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodeStakingOwnershipTransferred)
				if err := _NodeStaking.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_NodeStaking *NodeStakingFilterer) ParseOwnershipTransferred(log types.Log) (*NodeStakingOwnershipTransferred, error) {
	event := new(NodeStakingOwnershipTransferred)
	if err := _NodeStaking.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

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
	NodeAddress   common.Address
	StakedBalance *big.Int
	StakedCredits *big.Int
}

// NodeStakingMetaData contains all meta data concerning the NodeStaking contract.
var NodeStakingMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"creditsContract\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"benefitAddressContract\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"delegatedStakingContract\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"OwnableInvalidOwner\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"OwnableUnauthorizedAccount\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"stakedBalance\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"stakedCredits\",\"type\":\"uint256\"}],\"name\":\"NodeSlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"stakedBalance\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"stakedCredits\",\"type\":\"uint256\"}],\"name\":\"NodeStaked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"stakedBalance\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"stakedCredits\",\"type\":\"uint256\"}],\"name\":\"NodeUnstaked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"getAllNodeAddresses\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getMinStakeAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"getStakingInfo\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"stakedBalance\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"stakedCredits\",\"type\":\"uint256\"}],\"internalType\":\"structNodeStaking.StakingInfo\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"setAdminAddress\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"stakeAmount\",\"type\":\"uint256\"}],\"name\":\"setMinStakeAmount\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"slashStaking\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"stakedAmount\",\"type\":\"uint256\"}],\"name\":\"stake\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"unstake\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
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

// GetAllNodeAddresses is a free data retrieval call binding the contract method 0xc8fe3a01.
//
// Solidity: function getAllNodeAddresses() view returns(address[])
func (_NodeStaking *NodeStakingCaller) GetAllNodeAddresses(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _NodeStaking.contract.Call(opts, &out, "getAllNodeAddresses")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetAllNodeAddresses is a free data retrieval call binding the contract method 0xc8fe3a01.
//
// Solidity: function getAllNodeAddresses() view returns(address[])
func (_NodeStaking *NodeStakingSession) GetAllNodeAddresses() ([]common.Address, error) {
	return _NodeStaking.Contract.GetAllNodeAddresses(&_NodeStaking.CallOpts)
}

// GetAllNodeAddresses is a free data retrieval call binding the contract method 0xc8fe3a01.
//
// Solidity: function getAllNodeAddresses() view returns(address[])
func (_NodeStaking *NodeStakingCallerSession) GetAllNodeAddresses() ([]common.Address, error) {
	return _NodeStaking.Contract.GetAllNodeAddresses(&_NodeStaking.CallOpts)
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
// Solidity: function getStakingInfo(address nodeAddress) view returns((address,uint256,uint256))
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
// Solidity: function getStakingInfo(address nodeAddress) view returns((address,uint256,uint256))
func (_NodeStaking *NodeStakingSession) GetStakingInfo(nodeAddress common.Address) (NodeStakingStakingInfo, error) {
	return _NodeStaking.Contract.GetStakingInfo(&_NodeStaking.CallOpts, nodeAddress)
}

// GetStakingInfo is a free data retrieval call binding the contract method 0xaa4704f3.
//
// Solidity: function getStakingInfo(address nodeAddress) view returns((address,uint256,uint256))
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

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_NodeStaking *NodeStakingTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NodeStaking.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_NodeStaking *NodeStakingSession) RenounceOwnership() (*types.Transaction, error) {
	return _NodeStaking.Contract.RenounceOwnership(&_NodeStaking.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_NodeStaking *NodeStakingTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _NodeStaking.Contract.RenounceOwnership(&_NodeStaking.TransactOpts)
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
	StakedCredits *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterNodeSlashed is a free log retrieval operation binding the contract event 0xa8d720d0a0a2e7c96bf9eb87433901ebb6331356c8f3283b2568de34478703cc.
//
// Solidity: event NodeSlashed(address indexed nodeAddress, uint256 stakedBalance, uint256 stakedCredits)
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

// WatchNodeSlashed is a free log subscription operation binding the contract event 0xa8d720d0a0a2e7c96bf9eb87433901ebb6331356c8f3283b2568de34478703cc.
//
// Solidity: event NodeSlashed(address indexed nodeAddress, uint256 stakedBalance, uint256 stakedCredits)
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

// ParseNodeSlashed is a log parse operation binding the contract event 0xa8d720d0a0a2e7c96bf9eb87433901ebb6331356c8f3283b2568de34478703cc.
//
// Solidity: event NodeSlashed(address indexed nodeAddress, uint256 stakedBalance, uint256 stakedCredits)
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
	StakedCredits *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterNodeStaked is a free log retrieval operation binding the contract event 0x99a32eb4ddb5f70b97ac30749039f48ad104fff6c815cc699424d4ec8356224a.
//
// Solidity: event NodeStaked(address indexed nodeAddress, uint256 stakedBalance, uint256 stakedCredits)
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

// WatchNodeStaked is a free log subscription operation binding the contract event 0x99a32eb4ddb5f70b97ac30749039f48ad104fff6c815cc699424d4ec8356224a.
//
// Solidity: event NodeStaked(address indexed nodeAddress, uint256 stakedBalance, uint256 stakedCredits)
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

// ParseNodeStaked is a log parse operation binding the contract event 0x99a32eb4ddb5f70b97ac30749039f48ad104fff6c815cc699424d4ec8356224a.
//
// Solidity: event NodeStaked(address indexed nodeAddress, uint256 stakedBalance, uint256 stakedCredits)
func (_NodeStaking *NodeStakingFilterer) ParseNodeStaked(log types.Log) (*NodeStakingNodeStaked, error) {
	event := new(NodeStakingNodeStaked)
	if err := _NodeStaking.contract.UnpackLog(event, "NodeStaked", log); err != nil {
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
	StakedCredits *big.Int
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterNodeUnstaked is a free log retrieval operation binding the contract event 0xb4288416c9357d00b226d50e61ee6f3d6d7b94406ea7ede4c928a2dd3a4d0a0b.
//
// Solidity: event NodeUnstaked(address indexed nodeAddress, uint256 stakedBalance, uint256 stakedCredits)
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

// WatchNodeUnstaked is a free log subscription operation binding the contract event 0xb4288416c9357d00b226d50e61ee6f3d6d7b94406ea7ede4c928a2dd3a4d0a0b.
//
// Solidity: event NodeUnstaked(address indexed nodeAddress, uint256 stakedBalance, uint256 stakedCredits)
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

// ParseNodeUnstaked is a log parse operation binding the contract event 0xb4288416c9357d00b226d50e61ee6f3d6d7b94406ea7ede4c928a2dd3a4d0a0b.
//
// Solidity: event NodeUnstaked(address indexed nodeAddress, uint256 stakedBalance, uint256 stakedCredits)
func (_NodeStaking *NodeStakingFilterer) ParseNodeUnstaked(log types.Log) (*NodeStakingNodeUnstaked, error) {
	event := new(NodeStakingNodeUnstaked)
	if err := _NodeStaking.contract.UnpackLog(event, "NodeUnstaked", log); err != nil {
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

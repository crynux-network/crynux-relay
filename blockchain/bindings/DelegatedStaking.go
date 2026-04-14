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

// DelegatedStakingMetaData contains all meta data concerning the DelegatedStaking contract.
var DelegatedStakingMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"OwnableInvalidOwner\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"OwnableUnauthorizedAccount\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"delegatorAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"DelegatorStaked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"delegatorAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"DelegatorUnstaked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"share\",\"type\":\"uint8\"}],\"name\":\"NodeDelegatorShareChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"NodeSlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"getAllDelegatorAddresses\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getAllNodeAddresses\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getAllNodeDelegatorShares\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"},{\"internalType\":\"uint8[]\",\"name\":\"\",\"type\":\"uint8[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"delegatorAddress\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"getDelegationStakingAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"delegatorAddress\",\"type\":\"address\"}],\"name\":\"getDelegatorStakingInfos\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"delegatorAddress\",\"type\":\"address\"}],\"name\":\"getDelegatorTotalStakeAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getMinStakeAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"getNodeDelegatorShare\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"getNodeStakingInfos\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"getNodeTotalStakeAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"share\",\"type\":\"uint8\"}],\"name\":\"setDelegatorShare\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"stakeAmount\",\"type\":\"uint256\"}],\"name\":\"setMinStakeAmount\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"setNodeStakingAddress\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"slashNode\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"stake\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"unstake\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// DelegatedStakingABI is the input ABI used to generate the binding from.
// Deprecated: Use DelegatedStakingMetaData.ABI instead.
var DelegatedStakingABI = DelegatedStakingMetaData.ABI

// DelegatedStaking is an auto generated Go binding around an Ethereum contract.
type DelegatedStaking struct {
	DelegatedStakingCaller     // Read-only binding to the contract
	DelegatedStakingTransactor // Write-only binding to the contract
	DelegatedStakingFilterer   // Log filterer for contract events
}

// DelegatedStakingCaller is an auto generated read-only Go binding around an Ethereum contract.
type DelegatedStakingCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DelegatedStakingTransactor is an auto generated write-only Go binding around an Ethereum contract.
type DelegatedStakingTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DelegatedStakingFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type DelegatedStakingFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DelegatedStakingSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type DelegatedStakingSession struct {
	Contract     *DelegatedStaking // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// DelegatedStakingCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type DelegatedStakingCallerSession struct {
	Contract *DelegatedStakingCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// DelegatedStakingTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type DelegatedStakingTransactorSession struct {
	Contract     *DelegatedStakingTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// DelegatedStakingRaw is an auto generated low-level Go binding around an Ethereum contract.
type DelegatedStakingRaw struct {
	Contract *DelegatedStaking // Generic contract binding to access the raw methods on
}

// DelegatedStakingCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type DelegatedStakingCallerRaw struct {
	Contract *DelegatedStakingCaller // Generic read-only contract binding to access the raw methods on
}

// DelegatedStakingTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type DelegatedStakingTransactorRaw struct {
	Contract *DelegatedStakingTransactor // Generic write-only contract binding to access the raw methods on
}

// NewDelegatedStaking creates a new instance of DelegatedStaking, bound to a specific deployed contract.
func NewDelegatedStaking(address common.Address, backend bind.ContractBackend) (*DelegatedStaking, error) {
	contract, err := bindDelegatedStaking(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &DelegatedStaking{DelegatedStakingCaller: DelegatedStakingCaller{contract: contract}, DelegatedStakingTransactor: DelegatedStakingTransactor{contract: contract}, DelegatedStakingFilterer: DelegatedStakingFilterer{contract: contract}}, nil
}

// NewDelegatedStakingCaller creates a new read-only instance of DelegatedStaking, bound to a specific deployed contract.
func NewDelegatedStakingCaller(address common.Address, caller bind.ContractCaller) (*DelegatedStakingCaller, error) {
	contract, err := bindDelegatedStaking(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &DelegatedStakingCaller{contract: contract}, nil
}

// NewDelegatedStakingTransactor creates a new write-only instance of DelegatedStaking, bound to a specific deployed contract.
func NewDelegatedStakingTransactor(address common.Address, transactor bind.ContractTransactor) (*DelegatedStakingTransactor, error) {
	contract, err := bindDelegatedStaking(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &DelegatedStakingTransactor{contract: contract}, nil
}

// NewDelegatedStakingFilterer creates a new log filterer instance of DelegatedStaking, bound to a specific deployed contract.
func NewDelegatedStakingFilterer(address common.Address, filterer bind.ContractFilterer) (*DelegatedStakingFilterer, error) {
	contract, err := bindDelegatedStaking(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &DelegatedStakingFilterer{contract: contract}, nil
}

// bindDelegatedStaking binds a generic wrapper to an already deployed contract.
func bindDelegatedStaking(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := DelegatedStakingMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_DelegatedStaking *DelegatedStakingRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _DelegatedStaking.Contract.DelegatedStakingCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_DelegatedStaking *DelegatedStakingRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.DelegatedStakingTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_DelegatedStaking *DelegatedStakingRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.DelegatedStakingTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_DelegatedStaking *DelegatedStakingCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _DelegatedStaking.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_DelegatedStaking *DelegatedStakingTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_DelegatedStaking *DelegatedStakingTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.contract.Transact(opts, method, params...)
}

// GetAllDelegatorAddresses is a free data retrieval call binding the contract method 0x78340250.
//
// Solidity: function getAllDelegatorAddresses() view returns(address[])
func (_DelegatedStaking *DelegatedStakingCaller) GetAllDelegatorAddresses(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _DelegatedStaking.contract.Call(opts, &out, "getAllDelegatorAddresses")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetAllDelegatorAddresses is a free data retrieval call binding the contract method 0x78340250.
//
// Solidity: function getAllDelegatorAddresses() view returns(address[])
func (_DelegatedStaking *DelegatedStakingSession) GetAllDelegatorAddresses() ([]common.Address, error) {
	return _DelegatedStaking.Contract.GetAllDelegatorAddresses(&_DelegatedStaking.CallOpts)
}

// GetAllDelegatorAddresses is a free data retrieval call binding the contract method 0x78340250.
//
// Solidity: function getAllDelegatorAddresses() view returns(address[])
func (_DelegatedStaking *DelegatedStakingCallerSession) GetAllDelegatorAddresses() ([]common.Address, error) {
	return _DelegatedStaking.Contract.GetAllDelegatorAddresses(&_DelegatedStaking.CallOpts)
}

// GetAllNodeAddresses is a free data retrieval call binding the contract method 0xc8fe3a01.
//
// Solidity: function getAllNodeAddresses() view returns(address[])
func (_DelegatedStaking *DelegatedStakingCaller) GetAllNodeAddresses(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _DelegatedStaking.contract.Call(opts, &out, "getAllNodeAddresses")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetAllNodeAddresses is a free data retrieval call binding the contract method 0xc8fe3a01.
//
// Solidity: function getAllNodeAddresses() view returns(address[])
func (_DelegatedStaking *DelegatedStakingSession) GetAllNodeAddresses() ([]common.Address, error) {
	return _DelegatedStaking.Contract.GetAllNodeAddresses(&_DelegatedStaking.CallOpts)
}

// GetAllNodeAddresses is a free data retrieval call binding the contract method 0xc8fe3a01.
//
// Solidity: function getAllNodeAddresses() view returns(address[])
func (_DelegatedStaking *DelegatedStakingCallerSession) GetAllNodeAddresses() ([]common.Address, error) {
	return _DelegatedStaking.Contract.GetAllNodeAddresses(&_DelegatedStaking.CallOpts)
}

// GetAllNodeDelegatorShares is a free data retrieval call binding the contract method 0x20b2074f.
//
// Solidity: function getAllNodeDelegatorShares() view returns(address[], uint8[])
func (_DelegatedStaking *DelegatedStakingCaller) GetAllNodeDelegatorShares(opts *bind.CallOpts) ([]common.Address, []uint8, error) {
	var out []interface{}
	err := _DelegatedStaking.contract.Call(opts, &out, "getAllNodeDelegatorShares")

	if err != nil {
		return *new([]common.Address), *new([]uint8), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)
	out1 := *abi.ConvertType(out[1], new([]uint8)).(*[]uint8)

	return out0, out1, err

}

// GetAllNodeDelegatorShares is a free data retrieval call binding the contract method 0x20b2074f.
//
// Solidity: function getAllNodeDelegatorShares() view returns(address[], uint8[])
func (_DelegatedStaking *DelegatedStakingSession) GetAllNodeDelegatorShares() ([]common.Address, []uint8, error) {
	return _DelegatedStaking.Contract.GetAllNodeDelegatorShares(&_DelegatedStaking.CallOpts)
}

// GetAllNodeDelegatorShares is a free data retrieval call binding the contract method 0x20b2074f.
//
// Solidity: function getAllNodeDelegatorShares() view returns(address[], uint8[])
func (_DelegatedStaking *DelegatedStakingCallerSession) GetAllNodeDelegatorShares() ([]common.Address, []uint8, error) {
	return _DelegatedStaking.Contract.GetAllNodeDelegatorShares(&_DelegatedStaking.CallOpts)
}

// GetDelegationStakingAmount is a free data retrieval call binding the contract method 0x86ca0bc7.
//
// Solidity: function getDelegationStakingAmount(address delegatorAddress, address nodeAddress) view returns(uint256)
func (_DelegatedStaking *DelegatedStakingCaller) GetDelegationStakingAmount(opts *bind.CallOpts, delegatorAddress common.Address, nodeAddress common.Address) (*big.Int, error) {
	var out []interface{}
	err := _DelegatedStaking.contract.Call(opts, &out, "getDelegationStakingAmount", delegatorAddress, nodeAddress)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetDelegationStakingAmount is a free data retrieval call binding the contract method 0x86ca0bc7.
//
// Solidity: function getDelegationStakingAmount(address delegatorAddress, address nodeAddress) view returns(uint256)
func (_DelegatedStaking *DelegatedStakingSession) GetDelegationStakingAmount(delegatorAddress common.Address, nodeAddress common.Address) (*big.Int, error) {
	return _DelegatedStaking.Contract.GetDelegationStakingAmount(&_DelegatedStaking.CallOpts, delegatorAddress, nodeAddress)
}

// GetDelegationStakingAmount is a free data retrieval call binding the contract method 0x86ca0bc7.
//
// Solidity: function getDelegationStakingAmount(address delegatorAddress, address nodeAddress) view returns(uint256)
func (_DelegatedStaking *DelegatedStakingCallerSession) GetDelegationStakingAmount(delegatorAddress common.Address, nodeAddress common.Address) (*big.Int, error) {
	return _DelegatedStaking.Contract.GetDelegationStakingAmount(&_DelegatedStaking.CallOpts, delegatorAddress, nodeAddress)
}

// GetDelegatorStakingInfos is a free data retrieval call binding the contract method 0x71a1ba5a.
//
// Solidity: function getDelegatorStakingInfos(address delegatorAddress) view returns(address[], uint256[])
func (_DelegatedStaking *DelegatedStakingCaller) GetDelegatorStakingInfos(opts *bind.CallOpts, delegatorAddress common.Address) ([]common.Address, []*big.Int, error) {
	var out []interface{}
	err := _DelegatedStaking.contract.Call(opts, &out, "getDelegatorStakingInfos", delegatorAddress)

	if err != nil {
		return *new([]common.Address), *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)
	out1 := *abi.ConvertType(out[1], new([]*big.Int)).(*[]*big.Int)

	return out0, out1, err

}

// GetDelegatorStakingInfos is a free data retrieval call binding the contract method 0x71a1ba5a.
//
// Solidity: function getDelegatorStakingInfos(address delegatorAddress) view returns(address[], uint256[])
func (_DelegatedStaking *DelegatedStakingSession) GetDelegatorStakingInfos(delegatorAddress common.Address) ([]common.Address, []*big.Int, error) {
	return _DelegatedStaking.Contract.GetDelegatorStakingInfos(&_DelegatedStaking.CallOpts, delegatorAddress)
}

// GetDelegatorStakingInfos is a free data retrieval call binding the contract method 0x71a1ba5a.
//
// Solidity: function getDelegatorStakingInfos(address delegatorAddress) view returns(address[], uint256[])
func (_DelegatedStaking *DelegatedStakingCallerSession) GetDelegatorStakingInfos(delegatorAddress common.Address) ([]common.Address, []*big.Int, error) {
	return _DelegatedStaking.Contract.GetDelegatorStakingInfos(&_DelegatedStaking.CallOpts, delegatorAddress)
}

// GetDelegatorTotalStakeAmount is a free data retrieval call binding the contract method 0x33dc0f03.
//
// Solidity: function getDelegatorTotalStakeAmount(address delegatorAddress) view returns(uint256)
func (_DelegatedStaking *DelegatedStakingCaller) GetDelegatorTotalStakeAmount(opts *bind.CallOpts, delegatorAddress common.Address) (*big.Int, error) {
	var out []interface{}
	err := _DelegatedStaking.contract.Call(opts, &out, "getDelegatorTotalStakeAmount", delegatorAddress)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetDelegatorTotalStakeAmount is a free data retrieval call binding the contract method 0x33dc0f03.
//
// Solidity: function getDelegatorTotalStakeAmount(address delegatorAddress) view returns(uint256)
func (_DelegatedStaking *DelegatedStakingSession) GetDelegatorTotalStakeAmount(delegatorAddress common.Address) (*big.Int, error) {
	return _DelegatedStaking.Contract.GetDelegatorTotalStakeAmount(&_DelegatedStaking.CallOpts, delegatorAddress)
}

// GetDelegatorTotalStakeAmount is a free data retrieval call binding the contract method 0x33dc0f03.
//
// Solidity: function getDelegatorTotalStakeAmount(address delegatorAddress) view returns(uint256)
func (_DelegatedStaking *DelegatedStakingCallerSession) GetDelegatorTotalStakeAmount(delegatorAddress common.Address) (*big.Int, error) {
	return _DelegatedStaking.Contract.GetDelegatorTotalStakeAmount(&_DelegatedStaking.CallOpts, delegatorAddress)
}

// GetMinStakeAmount is a free data retrieval call binding the contract method 0x527cb1d7.
//
// Solidity: function getMinStakeAmount() view returns(uint256)
func (_DelegatedStaking *DelegatedStakingCaller) GetMinStakeAmount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _DelegatedStaking.contract.Call(opts, &out, "getMinStakeAmount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetMinStakeAmount is a free data retrieval call binding the contract method 0x527cb1d7.
//
// Solidity: function getMinStakeAmount() view returns(uint256)
func (_DelegatedStaking *DelegatedStakingSession) GetMinStakeAmount() (*big.Int, error) {
	return _DelegatedStaking.Contract.GetMinStakeAmount(&_DelegatedStaking.CallOpts)
}

// GetMinStakeAmount is a free data retrieval call binding the contract method 0x527cb1d7.
//
// Solidity: function getMinStakeAmount() view returns(uint256)
func (_DelegatedStaking *DelegatedStakingCallerSession) GetMinStakeAmount() (*big.Int, error) {
	return _DelegatedStaking.Contract.GetMinStakeAmount(&_DelegatedStaking.CallOpts)
}

// GetNodeDelegatorShare is a free data retrieval call binding the contract method 0x343c7202.
//
// Solidity: function getNodeDelegatorShare(address nodeAddress) view returns(uint8)
func (_DelegatedStaking *DelegatedStakingCaller) GetNodeDelegatorShare(opts *bind.CallOpts, nodeAddress common.Address) (uint8, error) {
	var out []interface{}
	err := _DelegatedStaking.contract.Call(opts, &out, "getNodeDelegatorShare", nodeAddress)

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// GetNodeDelegatorShare is a free data retrieval call binding the contract method 0x343c7202.
//
// Solidity: function getNodeDelegatorShare(address nodeAddress) view returns(uint8)
func (_DelegatedStaking *DelegatedStakingSession) GetNodeDelegatorShare(nodeAddress common.Address) (uint8, error) {
	return _DelegatedStaking.Contract.GetNodeDelegatorShare(&_DelegatedStaking.CallOpts, nodeAddress)
}

// GetNodeDelegatorShare is a free data retrieval call binding the contract method 0x343c7202.
//
// Solidity: function getNodeDelegatorShare(address nodeAddress) view returns(uint8)
func (_DelegatedStaking *DelegatedStakingCallerSession) GetNodeDelegatorShare(nodeAddress common.Address) (uint8, error) {
	return _DelegatedStaking.Contract.GetNodeDelegatorShare(&_DelegatedStaking.CallOpts, nodeAddress)
}

// GetNodeStakingInfos is a free data retrieval call binding the contract method 0xa54903f0.
//
// Solidity: function getNodeStakingInfos(address nodeAddress) view returns(address[], uint256[])
func (_DelegatedStaking *DelegatedStakingCaller) GetNodeStakingInfos(opts *bind.CallOpts, nodeAddress common.Address) ([]common.Address, []*big.Int, error) {
	var out []interface{}
	err := _DelegatedStaking.contract.Call(opts, &out, "getNodeStakingInfos", nodeAddress)

	if err != nil {
		return *new([]common.Address), *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)
	out1 := *abi.ConvertType(out[1], new([]*big.Int)).(*[]*big.Int)

	return out0, out1, err

}

// GetNodeStakingInfos is a free data retrieval call binding the contract method 0xa54903f0.
//
// Solidity: function getNodeStakingInfos(address nodeAddress) view returns(address[], uint256[])
func (_DelegatedStaking *DelegatedStakingSession) GetNodeStakingInfos(nodeAddress common.Address) ([]common.Address, []*big.Int, error) {
	return _DelegatedStaking.Contract.GetNodeStakingInfos(&_DelegatedStaking.CallOpts, nodeAddress)
}

// GetNodeStakingInfos is a free data retrieval call binding the contract method 0xa54903f0.
//
// Solidity: function getNodeStakingInfos(address nodeAddress) view returns(address[], uint256[])
func (_DelegatedStaking *DelegatedStakingCallerSession) GetNodeStakingInfos(nodeAddress common.Address) ([]common.Address, []*big.Int, error) {
	return _DelegatedStaking.Contract.GetNodeStakingInfos(&_DelegatedStaking.CallOpts, nodeAddress)
}

// GetNodeTotalStakeAmount is a free data retrieval call binding the contract method 0xc74ce32f.
//
// Solidity: function getNodeTotalStakeAmount(address nodeAddress) view returns(uint256)
func (_DelegatedStaking *DelegatedStakingCaller) GetNodeTotalStakeAmount(opts *bind.CallOpts, nodeAddress common.Address) (*big.Int, error) {
	var out []interface{}
	err := _DelegatedStaking.contract.Call(opts, &out, "getNodeTotalStakeAmount", nodeAddress)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetNodeTotalStakeAmount is a free data retrieval call binding the contract method 0xc74ce32f.
//
// Solidity: function getNodeTotalStakeAmount(address nodeAddress) view returns(uint256)
func (_DelegatedStaking *DelegatedStakingSession) GetNodeTotalStakeAmount(nodeAddress common.Address) (*big.Int, error) {
	return _DelegatedStaking.Contract.GetNodeTotalStakeAmount(&_DelegatedStaking.CallOpts, nodeAddress)
}

// GetNodeTotalStakeAmount is a free data retrieval call binding the contract method 0xc74ce32f.
//
// Solidity: function getNodeTotalStakeAmount(address nodeAddress) view returns(uint256)
func (_DelegatedStaking *DelegatedStakingCallerSession) GetNodeTotalStakeAmount(nodeAddress common.Address) (*big.Int, error) {
	return _DelegatedStaking.Contract.GetNodeTotalStakeAmount(&_DelegatedStaking.CallOpts, nodeAddress)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_DelegatedStaking *DelegatedStakingCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _DelegatedStaking.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_DelegatedStaking *DelegatedStakingSession) Owner() (common.Address, error) {
	return _DelegatedStaking.Contract.Owner(&_DelegatedStaking.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_DelegatedStaking *DelegatedStakingCallerSession) Owner() (common.Address, error) {
	return _DelegatedStaking.Contract.Owner(&_DelegatedStaking.CallOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_DelegatedStaking *DelegatedStakingTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DelegatedStaking.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_DelegatedStaking *DelegatedStakingSession) RenounceOwnership() (*types.Transaction, error) {
	return _DelegatedStaking.Contract.RenounceOwnership(&_DelegatedStaking.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_DelegatedStaking *DelegatedStakingTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _DelegatedStaking.Contract.RenounceOwnership(&_DelegatedStaking.TransactOpts)
}

// SetDelegatorShare is a paid mutator transaction binding the contract method 0x8a114c20.
//
// Solidity: function setDelegatorShare(uint8 share) returns()
func (_DelegatedStaking *DelegatedStakingTransactor) SetDelegatorShare(opts *bind.TransactOpts, share uint8) (*types.Transaction, error) {
	return _DelegatedStaking.contract.Transact(opts, "setDelegatorShare", share)
}

// SetDelegatorShare is a paid mutator transaction binding the contract method 0x8a114c20.
//
// Solidity: function setDelegatorShare(uint8 share) returns()
func (_DelegatedStaking *DelegatedStakingSession) SetDelegatorShare(share uint8) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.SetDelegatorShare(&_DelegatedStaking.TransactOpts, share)
}

// SetDelegatorShare is a paid mutator transaction binding the contract method 0x8a114c20.
//
// Solidity: function setDelegatorShare(uint8 share) returns()
func (_DelegatedStaking *DelegatedStakingTransactorSession) SetDelegatorShare(share uint8) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.SetDelegatorShare(&_DelegatedStaking.TransactOpts, share)
}

// SetMinStakeAmount is a paid mutator transaction binding the contract method 0xeb4af045.
//
// Solidity: function setMinStakeAmount(uint256 stakeAmount) returns()
func (_DelegatedStaking *DelegatedStakingTransactor) SetMinStakeAmount(opts *bind.TransactOpts, stakeAmount *big.Int) (*types.Transaction, error) {
	return _DelegatedStaking.contract.Transact(opts, "setMinStakeAmount", stakeAmount)
}

// SetMinStakeAmount is a paid mutator transaction binding the contract method 0xeb4af045.
//
// Solidity: function setMinStakeAmount(uint256 stakeAmount) returns()
func (_DelegatedStaking *DelegatedStakingSession) SetMinStakeAmount(stakeAmount *big.Int) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.SetMinStakeAmount(&_DelegatedStaking.TransactOpts, stakeAmount)
}

// SetMinStakeAmount is a paid mutator transaction binding the contract method 0xeb4af045.
//
// Solidity: function setMinStakeAmount(uint256 stakeAmount) returns()
func (_DelegatedStaking *DelegatedStakingTransactorSession) SetMinStakeAmount(stakeAmount *big.Int) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.SetMinStakeAmount(&_DelegatedStaking.TransactOpts, stakeAmount)
}

// SetNodeStakingAddress is a paid mutator transaction binding the contract method 0x6e970b58.
//
// Solidity: function setNodeStakingAddress(address addr) returns()
func (_DelegatedStaking *DelegatedStakingTransactor) SetNodeStakingAddress(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _DelegatedStaking.contract.Transact(opts, "setNodeStakingAddress", addr)
}

// SetNodeStakingAddress is a paid mutator transaction binding the contract method 0x6e970b58.
//
// Solidity: function setNodeStakingAddress(address addr) returns()
func (_DelegatedStaking *DelegatedStakingSession) SetNodeStakingAddress(addr common.Address) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.SetNodeStakingAddress(&_DelegatedStaking.TransactOpts, addr)
}

// SetNodeStakingAddress is a paid mutator transaction binding the contract method 0x6e970b58.
//
// Solidity: function setNodeStakingAddress(address addr) returns()
func (_DelegatedStaking *DelegatedStakingTransactorSession) SetNodeStakingAddress(addr common.Address) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.SetNodeStakingAddress(&_DelegatedStaking.TransactOpts, addr)
}

// SlashNode is a paid mutator transaction binding the contract method 0x5dca14f1.
//
// Solidity: function slashNode(address nodeAddress) returns()
func (_DelegatedStaking *DelegatedStakingTransactor) SlashNode(opts *bind.TransactOpts, nodeAddress common.Address) (*types.Transaction, error) {
	return _DelegatedStaking.contract.Transact(opts, "slashNode", nodeAddress)
}

// SlashNode is a paid mutator transaction binding the contract method 0x5dca14f1.
//
// Solidity: function slashNode(address nodeAddress) returns()
func (_DelegatedStaking *DelegatedStakingSession) SlashNode(nodeAddress common.Address) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.SlashNode(&_DelegatedStaking.TransactOpts, nodeAddress)
}

// SlashNode is a paid mutator transaction binding the contract method 0x5dca14f1.
//
// Solidity: function slashNode(address nodeAddress) returns()
func (_DelegatedStaking *DelegatedStakingTransactorSession) SlashNode(nodeAddress common.Address) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.SlashNode(&_DelegatedStaking.TransactOpts, nodeAddress)
}

// Stake is a paid mutator transaction binding the contract method 0xadc9772e.
//
// Solidity: function stake(address nodeAddress, uint256 amount) payable returns()
func (_DelegatedStaking *DelegatedStakingTransactor) Stake(opts *bind.TransactOpts, nodeAddress common.Address, amount *big.Int) (*types.Transaction, error) {
	return _DelegatedStaking.contract.Transact(opts, "stake", nodeAddress, amount)
}

// Stake is a paid mutator transaction binding the contract method 0xadc9772e.
//
// Solidity: function stake(address nodeAddress, uint256 amount) payable returns()
func (_DelegatedStaking *DelegatedStakingSession) Stake(nodeAddress common.Address, amount *big.Int) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.Stake(&_DelegatedStaking.TransactOpts, nodeAddress, amount)
}

// Stake is a paid mutator transaction binding the contract method 0xadc9772e.
//
// Solidity: function stake(address nodeAddress, uint256 amount) payable returns()
func (_DelegatedStaking *DelegatedStakingTransactorSession) Stake(nodeAddress common.Address, amount *big.Int) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.Stake(&_DelegatedStaking.TransactOpts, nodeAddress, amount)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_DelegatedStaking *DelegatedStakingTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _DelegatedStaking.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_DelegatedStaking *DelegatedStakingSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.TransferOwnership(&_DelegatedStaking.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_DelegatedStaking *DelegatedStakingTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.TransferOwnership(&_DelegatedStaking.TransactOpts, newOwner)
}

// Unstake is a paid mutator transaction binding the contract method 0xf2888dbb.
//
// Solidity: function unstake(address nodeAddress) returns()
func (_DelegatedStaking *DelegatedStakingTransactor) Unstake(opts *bind.TransactOpts, nodeAddress common.Address) (*types.Transaction, error) {
	return _DelegatedStaking.contract.Transact(opts, "unstake", nodeAddress)
}

// Unstake is a paid mutator transaction binding the contract method 0xf2888dbb.
//
// Solidity: function unstake(address nodeAddress) returns()
func (_DelegatedStaking *DelegatedStakingSession) Unstake(nodeAddress common.Address) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.Unstake(&_DelegatedStaking.TransactOpts, nodeAddress)
}

// Unstake is a paid mutator transaction binding the contract method 0xf2888dbb.
//
// Solidity: function unstake(address nodeAddress) returns()
func (_DelegatedStaking *DelegatedStakingTransactorSession) Unstake(nodeAddress common.Address) (*types.Transaction, error) {
	return _DelegatedStaking.Contract.Unstake(&_DelegatedStaking.TransactOpts, nodeAddress)
}

// DelegatedStakingDelegatorStakedIterator is returned from FilterDelegatorStaked and is used to iterate over the raw logs and unpacked data for DelegatorStaked events raised by the DelegatedStaking contract.
type DelegatedStakingDelegatorStakedIterator struct {
	Event *DelegatedStakingDelegatorStaked // Event containing the contract specifics and raw log

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
func (it *DelegatedStakingDelegatorStakedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DelegatedStakingDelegatorStaked)
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
		it.Event = new(DelegatedStakingDelegatorStaked)
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
func (it *DelegatedStakingDelegatorStakedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DelegatedStakingDelegatorStakedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DelegatedStakingDelegatorStaked represents a DelegatorStaked event raised by the DelegatedStaking contract.
type DelegatedStakingDelegatorStaked struct {
	DelegatorAddress common.Address
	NodeAddress      common.Address
	Amount           *big.Int
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterDelegatorStaked is a free log retrieval operation binding the contract event 0x1ed8ad98d928651a8bc3999999b718383931f4595fcd2e1efd2de972fa8cdaea.
//
// Solidity: event DelegatorStaked(address indexed delegatorAddress, address nodeAddress, uint256 amount)
func (_DelegatedStaking *DelegatedStakingFilterer) FilterDelegatorStaked(opts *bind.FilterOpts, delegatorAddress []common.Address) (*DelegatedStakingDelegatorStakedIterator, error) {

	var delegatorAddressRule []interface{}
	for _, delegatorAddressItem := range delegatorAddress {
		delegatorAddressRule = append(delegatorAddressRule, delegatorAddressItem)
	}

	logs, sub, err := _DelegatedStaking.contract.FilterLogs(opts, "DelegatorStaked", delegatorAddressRule)
	if err != nil {
		return nil, err
	}
	return &DelegatedStakingDelegatorStakedIterator{contract: _DelegatedStaking.contract, event: "DelegatorStaked", logs: logs, sub: sub}, nil
}

// WatchDelegatorStaked is a free log subscription operation binding the contract event 0x1ed8ad98d928651a8bc3999999b718383931f4595fcd2e1efd2de972fa8cdaea.
//
// Solidity: event DelegatorStaked(address indexed delegatorAddress, address nodeAddress, uint256 amount)
func (_DelegatedStaking *DelegatedStakingFilterer) WatchDelegatorStaked(opts *bind.WatchOpts, sink chan<- *DelegatedStakingDelegatorStaked, delegatorAddress []common.Address) (event.Subscription, error) {

	var delegatorAddressRule []interface{}
	for _, delegatorAddressItem := range delegatorAddress {
		delegatorAddressRule = append(delegatorAddressRule, delegatorAddressItem)
	}

	logs, sub, err := _DelegatedStaking.contract.WatchLogs(opts, "DelegatorStaked", delegatorAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DelegatedStakingDelegatorStaked)
				if err := _DelegatedStaking.contract.UnpackLog(event, "DelegatorStaked", log); err != nil {
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

// ParseDelegatorStaked is a log parse operation binding the contract event 0x1ed8ad98d928651a8bc3999999b718383931f4595fcd2e1efd2de972fa8cdaea.
//
// Solidity: event DelegatorStaked(address indexed delegatorAddress, address nodeAddress, uint256 amount)
func (_DelegatedStaking *DelegatedStakingFilterer) ParseDelegatorStaked(log types.Log) (*DelegatedStakingDelegatorStaked, error) {
	event := new(DelegatedStakingDelegatorStaked)
	if err := _DelegatedStaking.contract.UnpackLog(event, "DelegatorStaked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// DelegatedStakingDelegatorUnstakedIterator is returned from FilterDelegatorUnstaked and is used to iterate over the raw logs and unpacked data for DelegatorUnstaked events raised by the DelegatedStaking contract.
type DelegatedStakingDelegatorUnstakedIterator struct {
	Event *DelegatedStakingDelegatorUnstaked // Event containing the contract specifics and raw log

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
func (it *DelegatedStakingDelegatorUnstakedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DelegatedStakingDelegatorUnstaked)
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
		it.Event = new(DelegatedStakingDelegatorUnstaked)
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
func (it *DelegatedStakingDelegatorUnstakedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DelegatedStakingDelegatorUnstakedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DelegatedStakingDelegatorUnstaked represents a DelegatorUnstaked event raised by the DelegatedStaking contract.
type DelegatedStakingDelegatorUnstaked struct {
	DelegatorAddress common.Address
	NodeAddress      common.Address
	Amount           *big.Int
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterDelegatorUnstaked is a free log retrieval operation binding the contract event 0x01a8acbfce3387039b6b88cc7efd3b6b1cb826cc01f2524761becb1bb0cf4894.
//
// Solidity: event DelegatorUnstaked(address indexed delegatorAddress, address nodeAddress, uint256 amount)
func (_DelegatedStaking *DelegatedStakingFilterer) FilterDelegatorUnstaked(opts *bind.FilterOpts, delegatorAddress []common.Address) (*DelegatedStakingDelegatorUnstakedIterator, error) {

	var delegatorAddressRule []interface{}
	for _, delegatorAddressItem := range delegatorAddress {
		delegatorAddressRule = append(delegatorAddressRule, delegatorAddressItem)
	}

	logs, sub, err := _DelegatedStaking.contract.FilterLogs(opts, "DelegatorUnstaked", delegatorAddressRule)
	if err != nil {
		return nil, err
	}
	return &DelegatedStakingDelegatorUnstakedIterator{contract: _DelegatedStaking.contract, event: "DelegatorUnstaked", logs: logs, sub: sub}, nil
}

// WatchDelegatorUnstaked is a free log subscription operation binding the contract event 0x01a8acbfce3387039b6b88cc7efd3b6b1cb826cc01f2524761becb1bb0cf4894.
//
// Solidity: event DelegatorUnstaked(address indexed delegatorAddress, address nodeAddress, uint256 amount)
func (_DelegatedStaking *DelegatedStakingFilterer) WatchDelegatorUnstaked(opts *bind.WatchOpts, sink chan<- *DelegatedStakingDelegatorUnstaked, delegatorAddress []common.Address) (event.Subscription, error) {

	var delegatorAddressRule []interface{}
	for _, delegatorAddressItem := range delegatorAddress {
		delegatorAddressRule = append(delegatorAddressRule, delegatorAddressItem)
	}

	logs, sub, err := _DelegatedStaking.contract.WatchLogs(opts, "DelegatorUnstaked", delegatorAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DelegatedStakingDelegatorUnstaked)
				if err := _DelegatedStaking.contract.UnpackLog(event, "DelegatorUnstaked", log); err != nil {
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

// ParseDelegatorUnstaked is a log parse operation binding the contract event 0x01a8acbfce3387039b6b88cc7efd3b6b1cb826cc01f2524761becb1bb0cf4894.
//
// Solidity: event DelegatorUnstaked(address indexed delegatorAddress, address nodeAddress, uint256 amount)
func (_DelegatedStaking *DelegatedStakingFilterer) ParseDelegatorUnstaked(log types.Log) (*DelegatedStakingDelegatorUnstaked, error) {
	event := new(DelegatedStakingDelegatorUnstaked)
	if err := _DelegatedStaking.contract.UnpackLog(event, "DelegatorUnstaked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// DelegatedStakingNodeDelegatorShareChangedIterator is returned from FilterNodeDelegatorShareChanged and is used to iterate over the raw logs and unpacked data for NodeDelegatorShareChanged events raised by the DelegatedStaking contract.
type DelegatedStakingNodeDelegatorShareChangedIterator struct {
	Event *DelegatedStakingNodeDelegatorShareChanged // Event containing the contract specifics and raw log

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
func (it *DelegatedStakingNodeDelegatorShareChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DelegatedStakingNodeDelegatorShareChanged)
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
		it.Event = new(DelegatedStakingNodeDelegatorShareChanged)
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
func (it *DelegatedStakingNodeDelegatorShareChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DelegatedStakingNodeDelegatorShareChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DelegatedStakingNodeDelegatorShareChanged represents a NodeDelegatorShareChanged event raised by the DelegatedStaking contract.
type DelegatedStakingNodeDelegatorShareChanged struct {
	NodeAddress common.Address
	Share       uint8
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterNodeDelegatorShareChanged is a free log retrieval operation binding the contract event 0x5a6ed6932fa37b789945b6accca19be2a2e8b7f92485c26714cb8908e4c39cdb.
//
// Solidity: event NodeDelegatorShareChanged(address indexed nodeAddress, uint8 share)
func (_DelegatedStaking *DelegatedStakingFilterer) FilterNodeDelegatorShareChanged(opts *bind.FilterOpts, nodeAddress []common.Address) (*DelegatedStakingNodeDelegatorShareChangedIterator, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _DelegatedStaking.contract.FilterLogs(opts, "NodeDelegatorShareChanged", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return &DelegatedStakingNodeDelegatorShareChangedIterator{contract: _DelegatedStaking.contract, event: "NodeDelegatorShareChanged", logs: logs, sub: sub}, nil
}

// WatchNodeDelegatorShareChanged is a free log subscription operation binding the contract event 0x5a6ed6932fa37b789945b6accca19be2a2e8b7f92485c26714cb8908e4c39cdb.
//
// Solidity: event NodeDelegatorShareChanged(address indexed nodeAddress, uint8 share)
func (_DelegatedStaking *DelegatedStakingFilterer) WatchNodeDelegatorShareChanged(opts *bind.WatchOpts, sink chan<- *DelegatedStakingNodeDelegatorShareChanged, nodeAddress []common.Address) (event.Subscription, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _DelegatedStaking.contract.WatchLogs(opts, "NodeDelegatorShareChanged", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DelegatedStakingNodeDelegatorShareChanged)
				if err := _DelegatedStaking.contract.UnpackLog(event, "NodeDelegatorShareChanged", log); err != nil {
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

// ParseNodeDelegatorShareChanged is a log parse operation binding the contract event 0x5a6ed6932fa37b789945b6accca19be2a2e8b7f92485c26714cb8908e4c39cdb.
//
// Solidity: event NodeDelegatorShareChanged(address indexed nodeAddress, uint8 share)
func (_DelegatedStaking *DelegatedStakingFilterer) ParseNodeDelegatorShareChanged(log types.Log) (*DelegatedStakingNodeDelegatorShareChanged, error) {
	event := new(DelegatedStakingNodeDelegatorShareChanged)
	if err := _DelegatedStaking.contract.UnpackLog(event, "NodeDelegatorShareChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// DelegatedStakingNodeSlashedIterator is returned from FilterNodeSlashed and is used to iterate over the raw logs and unpacked data for NodeSlashed events raised by the DelegatedStaking contract.
type DelegatedStakingNodeSlashedIterator struct {
	Event *DelegatedStakingNodeSlashed // Event containing the contract specifics and raw log

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
func (it *DelegatedStakingNodeSlashedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DelegatedStakingNodeSlashed)
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
		it.Event = new(DelegatedStakingNodeSlashed)
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
func (it *DelegatedStakingNodeSlashedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DelegatedStakingNodeSlashedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DelegatedStakingNodeSlashed represents a NodeSlashed event raised by the DelegatedStaking contract.
type DelegatedStakingNodeSlashed struct {
	NodeAddress common.Address
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterNodeSlashed is a free log retrieval operation binding the contract event 0x29f3a9a9c7f6d4074ec8817742795e031d525ab8fe33b05ee339002580ef3a64.
//
// Solidity: event NodeSlashed(address indexed nodeAddress)
func (_DelegatedStaking *DelegatedStakingFilterer) FilterNodeSlashed(opts *bind.FilterOpts, nodeAddress []common.Address) (*DelegatedStakingNodeSlashedIterator, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _DelegatedStaking.contract.FilterLogs(opts, "NodeSlashed", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return &DelegatedStakingNodeSlashedIterator{contract: _DelegatedStaking.contract, event: "NodeSlashed", logs: logs, sub: sub}, nil
}

// WatchNodeSlashed is a free log subscription operation binding the contract event 0x29f3a9a9c7f6d4074ec8817742795e031d525ab8fe33b05ee339002580ef3a64.
//
// Solidity: event NodeSlashed(address indexed nodeAddress)
func (_DelegatedStaking *DelegatedStakingFilterer) WatchNodeSlashed(opts *bind.WatchOpts, sink chan<- *DelegatedStakingNodeSlashed, nodeAddress []common.Address) (event.Subscription, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _DelegatedStaking.contract.WatchLogs(opts, "NodeSlashed", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DelegatedStakingNodeSlashed)
				if err := _DelegatedStaking.contract.UnpackLog(event, "NodeSlashed", log); err != nil {
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

// ParseNodeSlashed is a log parse operation binding the contract event 0x29f3a9a9c7f6d4074ec8817742795e031d525ab8fe33b05ee339002580ef3a64.
//
// Solidity: event NodeSlashed(address indexed nodeAddress)
func (_DelegatedStaking *DelegatedStakingFilterer) ParseNodeSlashed(log types.Log) (*DelegatedStakingNodeSlashed, error) {
	event := new(DelegatedStakingNodeSlashed)
	if err := _DelegatedStaking.contract.UnpackLog(event, "NodeSlashed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// DelegatedStakingOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the DelegatedStaking contract.
type DelegatedStakingOwnershipTransferredIterator struct {
	Event *DelegatedStakingOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *DelegatedStakingOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DelegatedStakingOwnershipTransferred)
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
		it.Event = new(DelegatedStakingOwnershipTransferred)
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
func (it *DelegatedStakingOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DelegatedStakingOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DelegatedStakingOwnershipTransferred represents a OwnershipTransferred event raised by the DelegatedStaking contract.
type DelegatedStakingOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_DelegatedStaking *DelegatedStakingFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*DelegatedStakingOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _DelegatedStaking.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &DelegatedStakingOwnershipTransferredIterator{contract: _DelegatedStaking.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_DelegatedStaking *DelegatedStakingFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *DelegatedStakingOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _DelegatedStaking.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DelegatedStakingOwnershipTransferred)
				if err := _DelegatedStaking.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_DelegatedStaking *DelegatedStakingFilterer) ParseOwnershipTransferred(log types.Log) (*DelegatedStakingOwnershipTransferred, error) {
	event := new(DelegatedStakingOwnershipTransferred)
	if err := _DelegatedStaking.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

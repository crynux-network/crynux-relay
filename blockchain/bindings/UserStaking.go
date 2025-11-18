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

// UserStakingMetaData contains all meta data concerning the UserStaking contract.
var UserStakingMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"OwnableInvalidOwner\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"OwnableUnauthorizedAccount\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint8\",\"name\":\"share\",\"type\":\"uint8\"}],\"name\":\"NodeDelegatorShareChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"NodeSlashed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"userAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"UserStaked\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"userAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"UserUnstaked\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"getAllNodeAddresses\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getAllNodeDelegatorShares\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"},{\"internalType\":\"uint8[]\",\"name\":\"\",\"type\":\"uint8[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getAllUserAddresses\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getMinStakeAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"getNodeDelegatorShare\",\"outputs\":[{\"internalType\":\"uint8\",\"name\":\"\",\"type\":\"uint8\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"getNodeStakeAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"getNodeStakingInfos\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"userAddress\",\"type\":\"address\"}],\"name\":\"getUserStakeAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"userAddress\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"getUserStakingAmount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"userAddress\",\"type\":\"address\"}],\"name\":\"getUserStakingInfos\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint8\",\"name\":\"share\",\"type\":\"uint8\"}],\"name\":\"setDelegatorShare\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"stakeAmount\",\"type\":\"uint256\"}],\"name\":\"setMinStakeAmount\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"addr\",\"type\":\"address\"}],\"name\":\"setNodeStakingAddress\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"slashNode\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"stake\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"nodeAddress\",\"type\":\"address\"}],\"name\":\"unstake\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// UserStakingABI is the input ABI used to generate the binding from.
// Deprecated: Use UserStakingMetaData.ABI instead.
var UserStakingABI = UserStakingMetaData.ABI

// UserStaking is an auto generated Go binding around an Ethereum contract.
type UserStaking struct {
	UserStakingCaller     // Read-only binding to the contract
	UserStakingTransactor // Write-only binding to the contract
	UserStakingFilterer   // Log filterer for contract events
}

// UserStakingCaller is an auto generated read-only Go binding around an Ethereum contract.
type UserStakingCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// UserStakingTransactor is an auto generated write-only Go binding around an Ethereum contract.
type UserStakingTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// UserStakingFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type UserStakingFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// UserStakingSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type UserStakingSession struct {
	Contract     *UserStaking      // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// UserStakingCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type UserStakingCallerSession struct {
	Contract *UserStakingCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts      // Call options to use throughout this session
}

// UserStakingTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type UserStakingTransactorSession struct {
	Contract     *UserStakingTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts      // Transaction auth options to use throughout this session
}

// UserStakingRaw is an auto generated low-level Go binding around an Ethereum contract.
type UserStakingRaw struct {
	Contract *UserStaking // Generic contract binding to access the raw methods on
}

// UserStakingCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type UserStakingCallerRaw struct {
	Contract *UserStakingCaller // Generic read-only contract binding to access the raw methods on
}

// UserStakingTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type UserStakingTransactorRaw struct {
	Contract *UserStakingTransactor // Generic write-only contract binding to access the raw methods on
}

// NewUserStaking creates a new instance of UserStaking, bound to a specific deployed contract.
func NewUserStaking(address common.Address, backend bind.ContractBackend) (*UserStaking, error) {
	contract, err := bindUserStaking(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &UserStaking{UserStakingCaller: UserStakingCaller{contract: contract}, UserStakingTransactor: UserStakingTransactor{contract: contract}, UserStakingFilterer: UserStakingFilterer{contract: contract}}, nil
}

// NewUserStakingCaller creates a new read-only instance of UserStaking, bound to a specific deployed contract.
func NewUserStakingCaller(address common.Address, caller bind.ContractCaller) (*UserStakingCaller, error) {
	contract, err := bindUserStaking(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &UserStakingCaller{contract: contract}, nil
}

// NewUserStakingTransactor creates a new write-only instance of UserStaking, bound to a specific deployed contract.
func NewUserStakingTransactor(address common.Address, transactor bind.ContractTransactor) (*UserStakingTransactor, error) {
	contract, err := bindUserStaking(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &UserStakingTransactor{contract: contract}, nil
}

// NewUserStakingFilterer creates a new log filterer instance of UserStaking, bound to a specific deployed contract.
func NewUserStakingFilterer(address common.Address, filterer bind.ContractFilterer) (*UserStakingFilterer, error) {
	contract, err := bindUserStaking(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &UserStakingFilterer{contract: contract}, nil
}

// bindUserStaking binds a generic wrapper to an already deployed contract.
func bindUserStaking(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := UserStakingMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_UserStaking *UserStakingRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _UserStaking.Contract.UserStakingCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_UserStaking *UserStakingRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _UserStaking.Contract.UserStakingTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_UserStaking *UserStakingRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _UserStaking.Contract.UserStakingTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_UserStaking *UserStakingCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _UserStaking.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_UserStaking *UserStakingTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _UserStaking.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_UserStaking *UserStakingTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _UserStaking.Contract.contract.Transact(opts, method, params...)
}

// GetAllNodeAddresses is a free data retrieval call binding the contract method 0xc8fe3a01.
//
// Solidity: function getAllNodeAddresses() view returns(address[])
func (_UserStaking *UserStakingCaller) GetAllNodeAddresses(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _UserStaking.contract.Call(opts, &out, "getAllNodeAddresses")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetAllNodeAddresses is a free data retrieval call binding the contract method 0xc8fe3a01.
//
// Solidity: function getAllNodeAddresses() view returns(address[])
func (_UserStaking *UserStakingSession) GetAllNodeAddresses() ([]common.Address, error) {
	return _UserStaking.Contract.GetAllNodeAddresses(&_UserStaking.CallOpts)
}

// GetAllNodeAddresses is a free data retrieval call binding the contract method 0xc8fe3a01.
//
// Solidity: function getAllNodeAddresses() view returns(address[])
func (_UserStaking *UserStakingCallerSession) GetAllNodeAddresses() ([]common.Address, error) {
	return _UserStaking.Contract.GetAllNodeAddresses(&_UserStaking.CallOpts)
}

// GetAllNodeDelegatorShares is a free data retrieval call binding the contract method 0x20b2074f.
//
// Solidity: function getAllNodeDelegatorShares() view returns(address[], uint8[])
func (_UserStaking *UserStakingCaller) GetAllNodeDelegatorShares(opts *bind.CallOpts) ([]common.Address, []uint8, error) {
	var out []interface{}
	err := _UserStaking.contract.Call(opts, &out, "getAllNodeDelegatorShares")

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
func (_UserStaking *UserStakingSession) GetAllNodeDelegatorShares() ([]common.Address, []uint8, error) {
	return _UserStaking.Contract.GetAllNodeDelegatorShares(&_UserStaking.CallOpts)
}

// GetAllNodeDelegatorShares is a free data retrieval call binding the contract method 0x20b2074f.
//
// Solidity: function getAllNodeDelegatorShares() view returns(address[], uint8[])
func (_UserStaking *UserStakingCallerSession) GetAllNodeDelegatorShares() ([]common.Address, []uint8, error) {
	return _UserStaking.Contract.GetAllNodeDelegatorShares(&_UserStaking.CallOpts)
}

// GetAllUserAddresses is a free data retrieval call binding the contract method 0x2f330023.
//
// Solidity: function getAllUserAddresses() view returns(address[])
func (_UserStaking *UserStakingCaller) GetAllUserAddresses(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _UserStaking.contract.Call(opts, &out, "getAllUserAddresses")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetAllUserAddresses is a free data retrieval call binding the contract method 0x2f330023.
//
// Solidity: function getAllUserAddresses() view returns(address[])
func (_UserStaking *UserStakingSession) GetAllUserAddresses() ([]common.Address, error) {
	return _UserStaking.Contract.GetAllUserAddresses(&_UserStaking.CallOpts)
}

// GetAllUserAddresses is a free data retrieval call binding the contract method 0x2f330023.
//
// Solidity: function getAllUserAddresses() view returns(address[])
func (_UserStaking *UserStakingCallerSession) GetAllUserAddresses() ([]common.Address, error) {
	return _UserStaking.Contract.GetAllUserAddresses(&_UserStaking.CallOpts)
}

// GetMinStakeAmount is a free data retrieval call binding the contract method 0x527cb1d7.
//
// Solidity: function getMinStakeAmount() view returns(uint256)
func (_UserStaking *UserStakingCaller) GetMinStakeAmount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _UserStaking.contract.Call(opts, &out, "getMinStakeAmount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetMinStakeAmount is a free data retrieval call binding the contract method 0x527cb1d7.
//
// Solidity: function getMinStakeAmount() view returns(uint256)
func (_UserStaking *UserStakingSession) GetMinStakeAmount() (*big.Int, error) {
	return _UserStaking.Contract.GetMinStakeAmount(&_UserStaking.CallOpts)
}

// GetMinStakeAmount is a free data retrieval call binding the contract method 0x527cb1d7.
//
// Solidity: function getMinStakeAmount() view returns(uint256)
func (_UserStaking *UserStakingCallerSession) GetMinStakeAmount() (*big.Int, error) {
	return _UserStaking.Contract.GetMinStakeAmount(&_UserStaking.CallOpts)
}

// GetNodeDelegatorShare is a free data retrieval call binding the contract method 0x343c7202.
//
// Solidity: function getNodeDelegatorShare(address nodeAddress) view returns(uint8)
func (_UserStaking *UserStakingCaller) GetNodeDelegatorShare(opts *bind.CallOpts, nodeAddress common.Address) (uint8, error) {
	var out []interface{}
	err := _UserStaking.contract.Call(opts, &out, "getNodeDelegatorShare", nodeAddress)

	if err != nil {
		return *new(uint8), err
	}

	out0 := *abi.ConvertType(out[0], new(uint8)).(*uint8)

	return out0, err

}

// GetNodeDelegatorShare is a free data retrieval call binding the contract method 0x343c7202.
//
// Solidity: function getNodeDelegatorShare(address nodeAddress) view returns(uint8)
func (_UserStaking *UserStakingSession) GetNodeDelegatorShare(nodeAddress common.Address) (uint8, error) {
	return _UserStaking.Contract.GetNodeDelegatorShare(&_UserStaking.CallOpts, nodeAddress)
}

// GetNodeDelegatorShare is a free data retrieval call binding the contract method 0x343c7202.
//
// Solidity: function getNodeDelegatorShare(address nodeAddress) view returns(uint8)
func (_UserStaking *UserStakingCallerSession) GetNodeDelegatorShare(nodeAddress common.Address) (uint8, error) {
	return _UserStaking.Contract.GetNodeDelegatorShare(&_UserStaking.CallOpts, nodeAddress)
}

// GetNodeStakeAmount is a free data retrieval call binding the contract method 0x43e4b7dd.
//
// Solidity: function getNodeStakeAmount(address nodeAddress) view returns(uint256)
func (_UserStaking *UserStakingCaller) GetNodeStakeAmount(opts *bind.CallOpts, nodeAddress common.Address) (*big.Int, error) {
	var out []interface{}
	err := _UserStaking.contract.Call(opts, &out, "getNodeStakeAmount", nodeAddress)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetNodeStakeAmount is a free data retrieval call binding the contract method 0x43e4b7dd.
//
// Solidity: function getNodeStakeAmount(address nodeAddress) view returns(uint256)
func (_UserStaking *UserStakingSession) GetNodeStakeAmount(nodeAddress common.Address) (*big.Int, error) {
	return _UserStaking.Contract.GetNodeStakeAmount(&_UserStaking.CallOpts, nodeAddress)
}

// GetNodeStakeAmount is a free data retrieval call binding the contract method 0x43e4b7dd.
//
// Solidity: function getNodeStakeAmount(address nodeAddress) view returns(uint256)
func (_UserStaking *UserStakingCallerSession) GetNodeStakeAmount(nodeAddress common.Address) (*big.Int, error) {
	return _UserStaking.Contract.GetNodeStakeAmount(&_UserStaking.CallOpts, nodeAddress)
}

// GetNodeStakingInfos is a free data retrieval call binding the contract method 0xa54903f0.
//
// Solidity: function getNodeStakingInfos(address nodeAddress) view returns(address[], uint256[])
func (_UserStaking *UserStakingCaller) GetNodeStakingInfos(opts *bind.CallOpts, nodeAddress common.Address) ([]common.Address, []*big.Int, error) {
	var out []interface{}
	err := _UserStaking.contract.Call(opts, &out, "getNodeStakingInfos", nodeAddress)

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
func (_UserStaking *UserStakingSession) GetNodeStakingInfos(nodeAddress common.Address) ([]common.Address, []*big.Int, error) {
	return _UserStaking.Contract.GetNodeStakingInfos(&_UserStaking.CallOpts, nodeAddress)
}

// GetNodeStakingInfos is a free data retrieval call binding the contract method 0xa54903f0.
//
// Solidity: function getNodeStakingInfos(address nodeAddress) view returns(address[], uint256[])
func (_UserStaking *UserStakingCallerSession) GetNodeStakingInfos(nodeAddress common.Address) ([]common.Address, []*big.Int, error) {
	return _UserStaking.Contract.GetNodeStakingInfos(&_UserStaking.CallOpts, nodeAddress)
}

// GetUserStakeAmount is a free data retrieval call binding the contract method 0x7612f53c.
//
// Solidity: function getUserStakeAmount(address userAddress) view returns(uint256)
func (_UserStaking *UserStakingCaller) GetUserStakeAmount(opts *bind.CallOpts, userAddress common.Address) (*big.Int, error) {
	var out []interface{}
	err := _UserStaking.contract.Call(opts, &out, "getUserStakeAmount", userAddress)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetUserStakeAmount is a free data retrieval call binding the contract method 0x7612f53c.
//
// Solidity: function getUserStakeAmount(address userAddress) view returns(uint256)
func (_UserStaking *UserStakingSession) GetUserStakeAmount(userAddress common.Address) (*big.Int, error) {
	return _UserStaking.Contract.GetUserStakeAmount(&_UserStaking.CallOpts, userAddress)
}

// GetUserStakeAmount is a free data retrieval call binding the contract method 0x7612f53c.
//
// Solidity: function getUserStakeAmount(address userAddress) view returns(uint256)
func (_UserStaking *UserStakingCallerSession) GetUserStakeAmount(userAddress common.Address) (*big.Int, error) {
	return _UserStaking.Contract.GetUserStakeAmount(&_UserStaking.CallOpts, userAddress)
}

// GetUserStakingAmount is a free data retrieval call binding the contract method 0xeba94791.
//
// Solidity: function getUserStakingAmount(address userAddress, address nodeAddress) view returns(uint256)
func (_UserStaking *UserStakingCaller) GetUserStakingAmount(opts *bind.CallOpts, userAddress common.Address, nodeAddress common.Address) (*big.Int, error) {
	var out []interface{}
	err := _UserStaking.contract.Call(opts, &out, "getUserStakingAmount", userAddress, nodeAddress)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetUserStakingAmount is a free data retrieval call binding the contract method 0xeba94791.
//
// Solidity: function getUserStakingAmount(address userAddress, address nodeAddress) view returns(uint256)
func (_UserStaking *UserStakingSession) GetUserStakingAmount(userAddress common.Address, nodeAddress common.Address) (*big.Int, error) {
	return _UserStaking.Contract.GetUserStakingAmount(&_UserStaking.CallOpts, userAddress, nodeAddress)
}

// GetUserStakingAmount is a free data retrieval call binding the contract method 0xeba94791.
//
// Solidity: function getUserStakingAmount(address userAddress, address nodeAddress) view returns(uint256)
func (_UserStaking *UserStakingCallerSession) GetUserStakingAmount(userAddress common.Address, nodeAddress common.Address) (*big.Int, error) {
	return _UserStaking.Contract.GetUserStakingAmount(&_UserStaking.CallOpts, userAddress, nodeAddress)
}

// GetUserStakingInfos is a free data retrieval call binding the contract method 0x68b77d13.
//
// Solidity: function getUserStakingInfos(address userAddress) view returns(address[], uint256[])
func (_UserStaking *UserStakingCaller) GetUserStakingInfos(opts *bind.CallOpts, userAddress common.Address) ([]common.Address, []*big.Int, error) {
	var out []interface{}
	err := _UserStaking.contract.Call(opts, &out, "getUserStakingInfos", userAddress)

	if err != nil {
		return *new([]common.Address), *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)
	out1 := *abi.ConvertType(out[1], new([]*big.Int)).(*[]*big.Int)

	return out0, out1, err

}

// GetUserStakingInfos is a free data retrieval call binding the contract method 0x68b77d13.
//
// Solidity: function getUserStakingInfos(address userAddress) view returns(address[], uint256[])
func (_UserStaking *UserStakingSession) GetUserStakingInfos(userAddress common.Address) ([]common.Address, []*big.Int, error) {
	return _UserStaking.Contract.GetUserStakingInfos(&_UserStaking.CallOpts, userAddress)
}

// GetUserStakingInfos is a free data retrieval call binding the contract method 0x68b77d13.
//
// Solidity: function getUserStakingInfos(address userAddress) view returns(address[], uint256[])
func (_UserStaking *UserStakingCallerSession) GetUserStakingInfos(userAddress common.Address) ([]common.Address, []*big.Int, error) {
	return _UserStaking.Contract.GetUserStakingInfos(&_UserStaking.CallOpts, userAddress)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_UserStaking *UserStakingCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _UserStaking.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_UserStaking *UserStakingSession) Owner() (common.Address, error) {
	return _UserStaking.Contract.Owner(&_UserStaking.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_UserStaking *UserStakingCallerSession) Owner() (common.Address, error) {
	return _UserStaking.Contract.Owner(&_UserStaking.CallOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_UserStaking *UserStakingTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _UserStaking.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_UserStaking *UserStakingSession) RenounceOwnership() (*types.Transaction, error) {
	return _UserStaking.Contract.RenounceOwnership(&_UserStaking.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_UserStaking *UserStakingTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _UserStaking.Contract.RenounceOwnership(&_UserStaking.TransactOpts)
}

// SetDelegatorShare is a paid mutator transaction binding the contract method 0x8a114c20.
//
// Solidity: function setDelegatorShare(uint8 share) returns()
func (_UserStaking *UserStakingTransactor) SetDelegatorShare(opts *bind.TransactOpts, share uint8) (*types.Transaction, error) {
	return _UserStaking.contract.Transact(opts, "setDelegatorShare", share)
}

// SetDelegatorShare is a paid mutator transaction binding the contract method 0x8a114c20.
//
// Solidity: function setDelegatorShare(uint8 share) returns()
func (_UserStaking *UserStakingSession) SetDelegatorShare(share uint8) (*types.Transaction, error) {
	return _UserStaking.Contract.SetDelegatorShare(&_UserStaking.TransactOpts, share)
}

// SetDelegatorShare is a paid mutator transaction binding the contract method 0x8a114c20.
//
// Solidity: function setDelegatorShare(uint8 share) returns()
func (_UserStaking *UserStakingTransactorSession) SetDelegatorShare(share uint8) (*types.Transaction, error) {
	return _UserStaking.Contract.SetDelegatorShare(&_UserStaking.TransactOpts, share)
}

// SetMinStakeAmount is a paid mutator transaction binding the contract method 0xeb4af045.
//
// Solidity: function setMinStakeAmount(uint256 stakeAmount) returns()
func (_UserStaking *UserStakingTransactor) SetMinStakeAmount(opts *bind.TransactOpts, stakeAmount *big.Int) (*types.Transaction, error) {
	return _UserStaking.contract.Transact(opts, "setMinStakeAmount", stakeAmount)
}

// SetMinStakeAmount is a paid mutator transaction binding the contract method 0xeb4af045.
//
// Solidity: function setMinStakeAmount(uint256 stakeAmount) returns()
func (_UserStaking *UserStakingSession) SetMinStakeAmount(stakeAmount *big.Int) (*types.Transaction, error) {
	return _UserStaking.Contract.SetMinStakeAmount(&_UserStaking.TransactOpts, stakeAmount)
}

// SetMinStakeAmount is a paid mutator transaction binding the contract method 0xeb4af045.
//
// Solidity: function setMinStakeAmount(uint256 stakeAmount) returns()
func (_UserStaking *UserStakingTransactorSession) SetMinStakeAmount(stakeAmount *big.Int) (*types.Transaction, error) {
	return _UserStaking.Contract.SetMinStakeAmount(&_UserStaking.TransactOpts, stakeAmount)
}

// SetNodeStakingAddress is a paid mutator transaction binding the contract method 0x6e970b58.
//
// Solidity: function setNodeStakingAddress(address addr) returns()
func (_UserStaking *UserStakingTransactor) SetNodeStakingAddress(opts *bind.TransactOpts, addr common.Address) (*types.Transaction, error) {
	return _UserStaking.contract.Transact(opts, "setNodeStakingAddress", addr)
}

// SetNodeStakingAddress is a paid mutator transaction binding the contract method 0x6e970b58.
//
// Solidity: function setNodeStakingAddress(address addr) returns()
func (_UserStaking *UserStakingSession) SetNodeStakingAddress(addr common.Address) (*types.Transaction, error) {
	return _UserStaking.Contract.SetNodeStakingAddress(&_UserStaking.TransactOpts, addr)
}

// SetNodeStakingAddress is a paid mutator transaction binding the contract method 0x6e970b58.
//
// Solidity: function setNodeStakingAddress(address addr) returns()
func (_UserStaking *UserStakingTransactorSession) SetNodeStakingAddress(addr common.Address) (*types.Transaction, error) {
	return _UserStaking.Contract.SetNodeStakingAddress(&_UserStaking.TransactOpts, addr)
}

// SlashNode is a paid mutator transaction binding the contract method 0x5dca14f1.
//
// Solidity: function slashNode(address nodeAddress) returns()
func (_UserStaking *UserStakingTransactor) SlashNode(opts *bind.TransactOpts, nodeAddress common.Address) (*types.Transaction, error) {
	return _UserStaking.contract.Transact(opts, "slashNode", nodeAddress)
}

// SlashNode is a paid mutator transaction binding the contract method 0x5dca14f1.
//
// Solidity: function slashNode(address nodeAddress) returns()
func (_UserStaking *UserStakingSession) SlashNode(nodeAddress common.Address) (*types.Transaction, error) {
	return _UserStaking.Contract.SlashNode(&_UserStaking.TransactOpts, nodeAddress)
}

// SlashNode is a paid mutator transaction binding the contract method 0x5dca14f1.
//
// Solidity: function slashNode(address nodeAddress) returns()
func (_UserStaking *UserStakingTransactorSession) SlashNode(nodeAddress common.Address) (*types.Transaction, error) {
	return _UserStaking.Contract.SlashNode(&_UserStaking.TransactOpts, nodeAddress)
}

// Stake is a paid mutator transaction binding the contract method 0xadc9772e.
//
// Solidity: function stake(address nodeAddress, uint256 amount) payable returns()
func (_UserStaking *UserStakingTransactor) Stake(opts *bind.TransactOpts, nodeAddress common.Address, amount *big.Int) (*types.Transaction, error) {
	return _UserStaking.contract.Transact(opts, "stake", nodeAddress, amount)
}

// Stake is a paid mutator transaction binding the contract method 0xadc9772e.
//
// Solidity: function stake(address nodeAddress, uint256 amount) payable returns()
func (_UserStaking *UserStakingSession) Stake(nodeAddress common.Address, amount *big.Int) (*types.Transaction, error) {
	return _UserStaking.Contract.Stake(&_UserStaking.TransactOpts, nodeAddress, amount)
}

// Stake is a paid mutator transaction binding the contract method 0xadc9772e.
//
// Solidity: function stake(address nodeAddress, uint256 amount) payable returns()
func (_UserStaking *UserStakingTransactorSession) Stake(nodeAddress common.Address, amount *big.Int) (*types.Transaction, error) {
	return _UserStaking.Contract.Stake(&_UserStaking.TransactOpts, nodeAddress, amount)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_UserStaking *UserStakingTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _UserStaking.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_UserStaking *UserStakingSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _UserStaking.Contract.TransferOwnership(&_UserStaking.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_UserStaking *UserStakingTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _UserStaking.Contract.TransferOwnership(&_UserStaking.TransactOpts, newOwner)
}

// Unstake is a paid mutator transaction binding the contract method 0xf2888dbb.
//
// Solidity: function unstake(address nodeAddress) returns()
func (_UserStaking *UserStakingTransactor) Unstake(opts *bind.TransactOpts, nodeAddress common.Address) (*types.Transaction, error) {
	return _UserStaking.contract.Transact(opts, "unstake", nodeAddress)
}

// Unstake is a paid mutator transaction binding the contract method 0xf2888dbb.
//
// Solidity: function unstake(address nodeAddress) returns()
func (_UserStaking *UserStakingSession) Unstake(nodeAddress common.Address) (*types.Transaction, error) {
	return _UserStaking.Contract.Unstake(&_UserStaking.TransactOpts, nodeAddress)
}

// Unstake is a paid mutator transaction binding the contract method 0xf2888dbb.
//
// Solidity: function unstake(address nodeAddress) returns()
func (_UserStaking *UserStakingTransactorSession) Unstake(nodeAddress common.Address) (*types.Transaction, error) {
	return _UserStaking.Contract.Unstake(&_UserStaking.TransactOpts, nodeAddress)
}

// UserStakingNodeDelegatorShareChangedIterator is returned from FilterNodeDelegatorShareChanged and is used to iterate over the raw logs and unpacked data for NodeDelegatorShareChanged events raised by the UserStaking contract.
type UserStakingNodeDelegatorShareChangedIterator struct {
	Event *UserStakingNodeDelegatorShareChanged // Event containing the contract specifics and raw log

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
func (it *UserStakingNodeDelegatorShareChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(UserStakingNodeDelegatorShareChanged)
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
		it.Event = new(UserStakingNodeDelegatorShareChanged)
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
func (it *UserStakingNodeDelegatorShareChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *UserStakingNodeDelegatorShareChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// UserStakingNodeDelegatorShareChanged represents a NodeDelegatorShareChanged event raised by the UserStaking contract.
type UserStakingNodeDelegatorShareChanged struct {
	NodeAddress common.Address
	Share       uint8
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterNodeDelegatorShareChanged is a free log retrieval operation binding the contract event 0x5a6ed6932fa37b789945b6accca19be2a2e8b7f92485c26714cb8908e4c39cdb.
//
// Solidity: event NodeDelegatorShareChanged(address indexed nodeAddress, uint8 share)
func (_UserStaking *UserStakingFilterer) FilterNodeDelegatorShareChanged(opts *bind.FilterOpts, nodeAddress []common.Address) (*UserStakingNodeDelegatorShareChangedIterator, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _UserStaking.contract.FilterLogs(opts, "NodeDelegatorShareChanged", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return &UserStakingNodeDelegatorShareChangedIterator{contract: _UserStaking.contract, event: "NodeDelegatorShareChanged", logs: logs, sub: sub}, nil
}

// WatchNodeDelegatorShareChanged is a free log subscription operation binding the contract event 0x5a6ed6932fa37b789945b6accca19be2a2e8b7f92485c26714cb8908e4c39cdb.
//
// Solidity: event NodeDelegatorShareChanged(address indexed nodeAddress, uint8 share)
func (_UserStaking *UserStakingFilterer) WatchNodeDelegatorShareChanged(opts *bind.WatchOpts, sink chan<- *UserStakingNodeDelegatorShareChanged, nodeAddress []common.Address) (event.Subscription, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _UserStaking.contract.WatchLogs(opts, "NodeDelegatorShareChanged", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(UserStakingNodeDelegatorShareChanged)
				if err := _UserStaking.contract.UnpackLog(event, "NodeDelegatorShareChanged", log); err != nil {
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
func (_UserStaking *UserStakingFilterer) ParseNodeDelegatorShareChanged(log types.Log) (*UserStakingNodeDelegatorShareChanged, error) {
	event := new(UserStakingNodeDelegatorShareChanged)
	if err := _UserStaking.contract.UnpackLog(event, "NodeDelegatorShareChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// UserStakingNodeSlashedIterator is returned from FilterNodeSlashed and is used to iterate over the raw logs and unpacked data for NodeSlashed events raised by the UserStaking contract.
type UserStakingNodeSlashedIterator struct {
	Event *UserStakingNodeSlashed // Event containing the contract specifics and raw log

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
func (it *UserStakingNodeSlashedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(UserStakingNodeSlashed)
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
		it.Event = new(UserStakingNodeSlashed)
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
func (it *UserStakingNodeSlashedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *UserStakingNodeSlashedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// UserStakingNodeSlashed represents a NodeSlashed event raised by the UserStaking contract.
type UserStakingNodeSlashed struct {
	NodeAddress common.Address
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterNodeSlashed is a free log retrieval operation binding the contract event 0x29f3a9a9c7f6d4074ec8817742795e031d525ab8fe33b05ee339002580ef3a64.
//
// Solidity: event NodeSlashed(address indexed nodeAddress)
func (_UserStaking *UserStakingFilterer) FilterNodeSlashed(opts *bind.FilterOpts, nodeAddress []common.Address) (*UserStakingNodeSlashedIterator, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _UserStaking.contract.FilterLogs(opts, "NodeSlashed", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return &UserStakingNodeSlashedIterator{contract: _UserStaking.contract, event: "NodeSlashed", logs: logs, sub: sub}, nil
}

// WatchNodeSlashed is a free log subscription operation binding the contract event 0x29f3a9a9c7f6d4074ec8817742795e031d525ab8fe33b05ee339002580ef3a64.
//
// Solidity: event NodeSlashed(address indexed nodeAddress)
func (_UserStaking *UserStakingFilterer) WatchNodeSlashed(opts *bind.WatchOpts, sink chan<- *UserStakingNodeSlashed, nodeAddress []common.Address) (event.Subscription, error) {

	var nodeAddressRule []interface{}
	for _, nodeAddressItem := range nodeAddress {
		nodeAddressRule = append(nodeAddressRule, nodeAddressItem)
	}

	logs, sub, err := _UserStaking.contract.WatchLogs(opts, "NodeSlashed", nodeAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(UserStakingNodeSlashed)
				if err := _UserStaking.contract.UnpackLog(event, "NodeSlashed", log); err != nil {
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
func (_UserStaking *UserStakingFilterer) ParseNodeSlashed(log types.Log) (*UserStakingNodeSlashed, error) {
	event := new(UserStakingNodeSlashed)
	if err := _UserStaking.contract.UnpackLog(event, "NodeSlashed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// UserStakingOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the UserStaking contract.
type UserStakingOwnershipTransferredIterator struct {
	Event *UserStakingOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *UserStakingOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(UserStakingOwnershipTransferred)
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
		it.Event = new(UserStakingOwnershipTransferred)
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
func (it *UserStakingOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *UserStakingOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// UserStakingOwnershipTransferred represents a OwnershipTransferred event raised by the UserStaking contract.
type UserStakingOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_UserStaking *UserStakingFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*UserStakingOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _UserStaking.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &UserStakingOwnershipTransferredIterator{contract: _UserStaking.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_UserStaking *UserStakingFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *UserStakingOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _UserStaking.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(UserStakingOwnershipTransferred)
				if err := _UserStaking.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_UserStaking *UserStakingFilterer) ParseOwnershipTransferred(log types.Log) (*UserStakingOwnershipTransferred, error) {
	event := new(UserStakingOwnershipTransferred)
	if err := _UserStaking.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// UserStakingUserStakedIterator is returned from FilterUserStaked and is used to iterate over the raw logs and unpacked data for UserStaked events raised by the UserStaking contract.
type UserStakingUserStakedIterator struct {
	Event *UserStakingUserStaked // Event containing the contract specifics and raw log

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
func (it *UserStakingUserStakedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(UserStakingUserStaked)
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
		it.Event = new(UserStakingUserStaked)
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
func (it *UserStakingUserStakedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *UserStakingUserStakedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// UserStakingUserStaked represents a UserStaked event raised by the UserStaking contract.
type UserStakingUserStaked struct {
	UserAddress common.Address
	NodeAddress common.Address
	Amount      *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterUserStaked is a free log retrieval operation binding the contract event 0x2bf3a174b4957db8b38c0a9052b6e0b94ba046f022a2d940cb610f9d02bd381d.
//
// Solidity: event UserStaked(address indexed userAddress, address nodeAddress, uint256 amount)
func (_UserStaking *UserStakingFilterer) FilterUserStaked(opts *bind.FilterOpts, userAddress []common.Address) (*UserStakingUserStakedIterator, error) {

	var userAddressRule []interface{}
	for _, userAddressItem := range userAddress {
		userAddressRule = append(userAddressRule, userAddressItem)
	}

	logs, sub, err := _UserStaking.contract.FilterLogs(opts, "UserStaked", userAddressRule)
	if err != nil {
		return nil, err
	}
	return &UserStakingUserStakedIterator{contract: _UserStaking.contract, event: "UserStaked", logs: logs, sub: sub}, nil
}

// WatchUserStaked is a free log subscription operation binding the contract event 0x2bf3a174b4957db8b38c0a9052b6e0b94ba046f022a2d940cb610f9d02bd381d.
//
// Solidity: event UserStaked(address indexed userAddress, address nodeAddress, uint256 amount)
func (_UserStaking *UserStakingFilterer) WatchUserStaked(opts *bind.WatchOpts, sink chan<- *UserStakingUserStaked, userAddress []common.Address) (event.Subscription, error) {

	var userAddressRule []interface{}
	for _, userAddressItem := range userAddress {
		userAddressRule = append(userAddressRule, userAddressItem)
	}

	logs, sub, err := _UserStaking.contract.WatchLogs(opts, "UserStaked", userAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(UserStakingUserStaked)
				if err := _UserStaking.contract.UnpackLog(event, "UserStaked", log); err != nil {
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

// ParseUserStaked is a log parse operation binding the contract event 0x2bf3a174b4957db8b38c0a9052b6e0b94ba046f022a2d940cb610f9d02bd381d.
//
// Solidity: event UserStaked(address indexed userAddress, address nodeAddress, uint256 amount)
func (_UserStaking *UserStakingFilterer) ParseUserStaked(log types.Log) (*UserStakingUserStaked, error) {
	event := new(UserStakingUserStaked)
	if err := _UserStaking.contract.UnpackLog(event, "UserStaked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// UserStakingUserUnstakedIterator is returned from FilterUserUnstaked and is used to iterate over the raw logs and unpacked data for UserUnstaked events raised by the UserStaking contract.
type UserStakingUserUnstakedIterator struct {
	Event *UserStakingUserUnstaked // Event containing the contract specifics and raw log

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
func (it *UserStakingUserUnstakedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(UserStakingUserUnstaked)
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
		it.Event = new(UserStakingUserUnstaked)
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
func (it *UserStakingUserUnstakedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *UserStakingUserUnstakedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// UserStakingUserUnstaked represents a UserUnstaked event raised by the UserStaking contract.
type UserStakingUserUnstaked struct {
	UserAddress common.Address
	NodeAddress common.Address
	Amount      *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterUserUnstaked is a free log retrieval operation binding the contract event 0xc7151c3d5a29ed0e449206cedd0a4d7681c36474de06b49d60a28404d52d5125.
//
// Solidity: event UserUnstaked(address indexed userAddress, address nodeAddress, uint256 amount)
func (_UserStaking *UserStakingFilterer) FilterUserUnstaked(opts *bind.FilterOpts, userAddress []common.Address) (*UserStakingUserUnstakedIterator, error) {

	var userAddressRule []interface{}
	for _, userAddressItem := range userAddress {
		userAddressRule = append(userAddressRule, userAddressItem)
	}

	logs, sub, err := _UserStaking.contract.FilterLogs(opts, "UserUnstaked", userAddressRule)
	if err != nil {
		return nil, err
	}
	return &UserStakingUserUnstakedIterator{contract: _UserStaking.contract, event: "UserUnstaked", logs: logs, sub: sub}, nil
}

// WatchUserUnstaked is a free log subscription operation binding the contract event 0xc7151c3d5a29ed0e449206cedd0a4d7681c36474de06b49d60a28404d52d5125.
//
// Solidity: event UserUnstaked(address indexed userAddress, address nodeAddress, uint256 amount)
func (_UserStaking *UserStakingFilterer) WatchUserUnstaked(opts *bind.WatchOpts, sink chan<- *UserStakingUserUnstaked, userAddress []common.Address) (event.Subscription, error) {

	var userAddressRule []interface{}
	for _, userAddressItem := range userAddress {
		userAddressRule = append(userAddressRule, userAddressItem)
	}

	logs, sub, err := _UserStaking.contract.WatchLogs(opts, "UserUnstaked", userAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(UserStakingUserUnstaked)
				if err := _UserStaking.contract.UnpackLog(event, "UserUnstaked", log); err != nil {
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

// ParseUserUnstaked is a log parse operation binding the contract event 0xc7151c3d5a29ed0e449206cedd0a4d7681c36474de06b49d60a28404d52d5125.
//
// Solidity: event UserUnstaked(address indexed userAddress, address nodeAddress, uint256 amount)
func (_UserStaking *UserStakingFilterer) ParseUserUnstaked(log types.Log) (*UserStakingUserUnstaked, error) {
	event := new(UserStakingUserUnstaked)
	if err := _UserStaking.contract.UnpackLog(event, "UserUnstaked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

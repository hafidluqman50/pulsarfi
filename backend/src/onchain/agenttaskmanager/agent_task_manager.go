// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package agenttaskmanager

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

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
	_ = time.Tick
	_ = context.Background
)

// AgentTaskManagerSubTaskRecord is an auto generated low-level Go binding around an user-defined struct.
type AgentTaskManagerSubTaskRecord struct {
	TaskId               *big.Int
	Agent                uint8
	StepName             string
	Status               uint8
	RoutingTarget        string
	Summary              string
	ReasoningHash        [32]byte
	OutputHash           [32]byte
	DecisionHash         [32]byte
	PreviousDecisionHash [32]byte
}

// AgentTaskManagerMetaData contains all meta data concerning the AgentTaskManager contract.
var AgentTaskManagerMetaData = &bind.MetaData{
	ABI: "[{\"type\":\"constructor\",\"inputs\":[{\"name\":\"protocol_\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"admin\",\"type\":\"address\",\"internalType\":\"address\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"AGENT_ROLE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"DEFAULT_ADMIN_ROLE\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"cancelTask\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"createTask\",\"inputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"isActionable\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"summary\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"promptHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"executeTrade\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"subTaskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"ticker\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"side\",\"type\":\"uint8\",\"internalType\":\"enumAgentTaskManager.TradeSide\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"minimumOutputAmount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"summary\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"reasoningHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"tradeId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"getRoleAdmin\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"grantRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"grantTradePermission\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"totalBudget\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"duration\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"hasRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"idrx\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIERC20\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"isActive\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"protocol\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"address\",\"internalType\":\"contractIPulsarProtocolForAgent\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"recordSubTasks\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"subTaskRecords\",\"type\":\"tuple[]\",\"internalType\":\"structAgentTaskManager.SubTaskRecord[]\",\"components\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"agent\",\"type\":\"uint8\",\"internalType\":\"enumAgentTaskManager.TaskAgent\"},{\"name\":\"stepName\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumAgentTaskManager.SubTaskStatus\"},{\"name\":\"routingTarget\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"summary\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"reasoningHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"outputHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"decisionHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"previousDecisionHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]}],\"outputs\":[{\"name\":\"subTaskIds\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"}],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"renounceRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"callerConfirmation\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"revokeRole\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"}],\"outputs\":[],\"stateMutability\":\"nonpayable\"},{\"type\":\"function\",\"name\":\"subTaskCount\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"subTaskIdsByTaskId\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"subTaskIds\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"subTaskIdsForTask\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"subTasks\",\"inputs\":[{\"name\":\"subTaskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"agent\",\"type\":\"uint8\",\"internalType\":\"enumAgentTaskManager.TaskAgent\"},{\"name\":\"stepName\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumAgentTaskManager.SubTaskStatus\"},{\"name\":\"routingTarget\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"summary\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"reasoningHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"outputHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"decisionHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"previousDecisionHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"supportsInterface\",\"inputs\":[{\"name\":\"interfaceId\",\"type\":\"bytes4\",\"internalType\":\"bytes4\"}],\"outputs\":[{\"name\":\"\",\"type\":\"bool\",\"internalType\":\"bool\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"taskCount\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"tasks\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"owner\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"isActionable\",\"type\":\"bool\",\"internalType\":\"bool\"},{\"name\":\"status\",\"type\":\"uint8\",\"internalType\":\"enumAgentTaskManager.TaskStatus\"},{\"name\":\"summary\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"promptHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"tradeCount\",\"inputs\":[],\"outputs\":[{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"tradeIdsByTaskId\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"tradeIds\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"tradeIdsForTask\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"\",\"type\":\"uint256[]\",\"internalType\":\"uint256[]\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"tradePermissions\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"totalBudget\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"usedBudget\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"expiresAt\",\"type\":\"uint64\",\"internalType\":\"uint64\"}],\"stateMutability\":\"view\"},{\"type\":\"function\",\"name\":\"trades\",\"inputs\":[{\"name\":\"tradeId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"outputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"subTaskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"ticker\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"side\",\"type\":\"uint8\",\"internalType\":\"enumAgentTaskManager.TradeSide\"},{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"receivedAmount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"summary\",\"type\":\"string\",\"internalType\":\"string\"},{\"name\":\"reasoningHash\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"},{\"name\":\"executedAt\",\"type\":\"uint256\",\"internalType\":\"uint256\"}],\"stateMutability\":\"view\"},{\"type\":\"event\",\"name\":\"RoleAdminChanged\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"previousAdminRole\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"newAdminRole\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RoleGranted\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"sender\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"RoleRevoked\",\"inputs\":[{\"name\":\"role\",\"type\":\"bytes32\",\"indexed\":true,\"internalType\":\"bytes32\"},{\"name\":\"account\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"sender\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"SubTaskRecorded\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"subTaskId\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"agent\",\"type\":\"uint8\",\"indexed\":true,\"internalType\":\"enumAgentTaskManager.TaskAgent\"},{\"name\":\"stepName\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"},{\"name\":\"summary\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"TaskCancelled\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"cancelledBy\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"TaskCreated\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"owner\",\"type\":\"address\",\"indexed\":true,\"internalType\":\"address\"},{\"name\":\"isActionable\",\"type\":\"bool\",\"indexed\":false,\"internalType\":\"bool\"},{\"name\":\"summary\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"TradeExecuted\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"tradeId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"ticker\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"},{\"name\":\"side\",\"type\":\"uint8\",\"indexed\":false,\"internalType\":\"enumAgentTaskManager.TradeSide\"},{\"name\":\"amount\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"summary\",\"type\":\"string\",\"indexed\":false,\"internalType\":\"string\"}],\"anonymous\":false},{\"type\":\"event\",\"name\":\"TradePermissionGranted\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"indexed\":true,\"internalType\":\"uint256\"},{\"name\":\"totalBudget\",\"type\":\"uint256\",\"indexed\":false,\"internalType\":\"uint256\"},{\"name\":\"expiresAt\",\"type\":\"uint64\",\"indexed\":false,\"internalType\":\"uint64\"}],\"anonymous\":false},{\"type\":\"error\",\"name\":\"AccessControlBadConfirmation\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"AccessControlUnauthorizedAccount\",\"inputs\":[{\"name\":\"account\",\"type\":\"address\",\"internalType\":\"address\"},{\"name\":\"neededRole\",\"type\":\"bytes32\",\"internalType\":\"bytes32\"}]},{\"type\":\"error\",\"name\":\"AllowanceExceeded\",\"inputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"allowance\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"BudgetExceeded\",\"inputs\":[{\"name\":\"amount\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"remaining\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"NoTradePermission\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"NotOwnerOrAgent\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"caller\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"ReentrancyGuardReentrantCall\",\"inputs\":[]},{\"type\":\"error\",\"name\":\"SafeERC20FailedOperation\",\"inputs\":[{\"name\":\"token\",\"type\":\"address\",\"internalType\":\"address\"}]},{\"type\":\"error\",\"name\":\"SubTaskNotFound\",\"inputs\":[{\"name\":\"subTaskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"SubTaskTaskMismatch\",\"inputs\":[{\"name\":\"subTaskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"expectedTaskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"TaskNotActionable\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"TaskNotActive\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"TaskNotFound\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"TradePermissionAlreadyGranted\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"}]},{\"type\":\"error\",\"name\":\"TradePermissionExpired\",\"inputs\":[{\"name\":\"taskId\",\"type\":\"uint256\",\"internalType\":\"uint256\"},{\"name\":\"expiresAt\",\"type\":\"uint64\",\"internalType\":\"uint64\"}]},{\"type\":\"error\",\"name\":\"ZeroAddress\",\"inputs\":[]}]",
}

// AgentTaskManagerABI is the input ABI used to generate the binding from.
// Deprecated: Use AgentTaskManagerMetaData.ABI instead.
var AgentTaskManagerABI = AgentTaskManagerMetaData.ABI

// AgentTaskManager is an auto generated Go binding around an Ethereum contract.
type AgentTaskManager struct {
	AgentTaskManagerCaller     // Read-only binding to the contract
	AgentTaskManagerTransactor // Write-only binding to the contract
	AgentTaskManagerFilterer   // Log filterer for contract events
}

// AgentTaskManagerCaller is an auto generated read-only Go binding around an Ethereum contract.
type AgentTaskManagerCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// AgentTaskManagerTransactor is an auto generated write-only Go binding around an Ethereum contract.
type AgentTaskManagerTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// AgentTaskManagerFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type AgentTaskManagerFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// AgentTaskManagerSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type AgentTaskManagerSession struct {
	Contract     *AgentTaskManager // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// AgentTaskManagerCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type AgentTaskManagerCallerSession struct {
	Contract *AgentTaskManagerCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// AgentTaskManagerTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type AgentTaskManagerTransactorSession struct {
	Contract     *AgentTaskManagerTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// AgentTaskManagerRaw is an auto generated low-level Go binding around an Ethereum contract.
type AgentTaskManagerRaw struct {
	Contract *AgentTaskManager // Generic contract binding to access the raw methods on
}

// AgentTaskManagerCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type AgentTaskManagerCallerRaw struct {
	Contract *AgentTaskManagerCaller // Generic read-only contract binding to access the raw methods on
}

// AgentTaskManagerTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type AgentTaskManagerTransactorRaw struct {
	Contract *AgentTaskManagerTransactor // Generic write-only contract binding to access the raw methods on
}

// NewAgentTaskManager creates a new instance of AgentTaskManager, bound to a specific deployed contract.
func NewAgentTaskManager(address common.Address, backend bind.ContractBackend) (*AgentTaskManager, error) {
	contract, err := bindAgentTaskManager(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &AgentTaskManager{AgentTaskManagerCaller: AgentTaskManagerCaller{contract: contract}, AgentTaskManagerTransactor: AgentTaskManagerTransactor{contract: contract}, AgentTaskManagerFilterer: AgentTaskManagerFilterer{contract: contract}}, nil
}

// NewAgentTaskManagerCaller creates a new read-only instance of AgentTaskManager, bound to a specific deployed contract.
func NewAgentTaskManagerCaller(address common.Address, caller bind.ContractCaller) (*AgentTaskManagerCaller, error) {
	contract, err := bindAgentTaskManager(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &AgentTaskManagerCaller{contract: contract}, nil
}

// NewAgentTaskManagerTransactor creates a new write-only instance of AgentTaskManager, bound to a specific deployed contract.
func NewAgentTaskManagerTransactor(address common.Address, transactor bind.ContractTransactor) (*AgentTaskManagerTransactor, error) {
	contract, err := bindAgentTaskManager(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &AgentTaskManagerTransactor{contract: contract}, nil
}

// NewAgentTaskManagerFilterer creates a new log filterer instance of AgentTaskManager, bound to a specific deployed contract.
func NewAgentTaskManagerFilterer(address common.Address, filterer bind.ContractFilterer) (*AgentTaskManagerFilterer, error) {
	contract, err := bindAgentTaskManager(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &AgentTaskManagerFilterer{contract: contract}, nil
}

// bindAgentTaskManager binds a generic wrapper to an already deployed contract.
func bindAgentTaskManager(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := AgentTaskManagerMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_AgentTaskManager *AgentTaskManagerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _AgentTaskManager.Contract.AgentTaskManagerCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_AgentTaskManager *AgentTaskManagerRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.AgentTaskManagerTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_AgentTaskManager *AgentTaskManagerRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.AgentTaskManagerTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_AgentTaskManager *AgentTaskManagerCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _AgentTaskManager.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_AgentTaskManager *AgentTaskManagerTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_AgentTaskManager *AgentTaskManagerTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.contract.Transact(opts, method, params...)
}

// AGENTROLE is a free data retrieval call binding the contract method 0x22459e18.
//
// Solidity: function AGENT_ROLE() view returns(bytes32)
func (_AgentTaskManager *AgentTaskManagerCaller) AGENTROLE(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "AGENT_ROLE")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// AGENTROLE is a free data retrieval call binding the contract method 0x22459e18.
//
// Solidity: function AGENT_ROLE() view returns(bytes32)
func (_AgentTaskManager *AgentTaskManagerSession) AGENTROLE() ([32]byte, error) {
	return _AgentTaskManager.Contract.AGENTROLE(&_AgentTaskManager.CallOpts)
}

// AGENTROLE is a free data retrieval call binding the contract method 0x22459e18.
//
// Solidity: function AGENT_ROLE() view returns(bytes32)
func (_AgentTaskManager *AgentTaskManagerCallerSession) AGENTROLE() ([32]byte, error) {
	return _AgentTaskManager.Contract.AGENTROLE(&_AgentTaskManager.CallOpts)
}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_AgentTaskManager *AgentTaskManagerCaller) DEFAULTADMINROLE(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "DEFAULT_ADMIN_ROLE")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_AgentTaskManager *AgentTaskManagerSession) DEFAULTADMINROLE() ([32]byte, error) {
	return _AgentTaskManager.Contract.DEFAULTADMINROLE(&_AgentTaskManager.CallOpts)
}

// DEFAULTADMINROLE is a free data retrieval call binding the contract method 0xa217fddf.
//
// Solidity: function DEFAULT_ADMIN_ROLE() view returns(bytes32)
func (_AgentTaskManager *AgentTaskManagerCallerSession) DEFAULTADMINROLE() ([32]byte, error) {
	return _AgentTaskManager.Contract.DEFAULTADMINROLE(&_AgentTaskManager.CallOpts)
}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_AgentTaskManager *AgentTaskManagerCaller) GetRoleAdmin(opts *bind.CallOpts, role [32]byte) ([32]byte, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "getRoleAdmin", role)

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_AgentTaskManager *AgentTaskManagerSession) GetRoleAdmin(role [32]byte) ([32]byte, error) {
	return _AgentTaskManager.Contract.GetRoleAdmin(&_AgentTaskManager.CallOpts, role)
}

// GetRoleAdmin is a free data retrieval call binding the contract method 0x248a9ca3.
//
// Solidity: function getRoleAdmin(bytes32 role) view returns(bytes32)
func (_AgentTaskManager *AgentTaskManagerCallerSession) GetRoleAdmin(role [32]byte) ([32]byte, error) {
	return _AgentTaskManager.Contract.GetRoleAdmin(&_AgentTaskManager.CallOpts, role)
}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_AgentTaskManager *AgentTaskManagerCaller) HasRole(opts *bind.CallOpts, role [32]byte, account common.Address) (bool, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "hasRole", role, account)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_AgentTaskManager *AgentTaskManagerSession) HasRole(role [32]byte, account common.Address) (bool, error) {
	return _AgentTaskManager.Contract.HasRole(&_AgentTaskManager.CallOpts, role, account)
}

// HasRole is a free data retrieval call binding the contract method 0x91d14854.
//
// Solidity: function hasRole(bytes32 role, address account) view returns(bool)
func (_AgentTaskManager *AgentTaskManagerCallerSession) HasRole(role [32]byte, account common.Address) (bool, error) {
	return _AgentTaskManager.Contract.HasRole(&_AgentTaskManager.CallOpts, role, account)
}

// Idrx is a free data retrieval call binding the contract method 0xe172c609.
//
// Solidity: function idrx() view returns(address)
func (_AgentTaskManager *AgentTaskManagerCaller) Idrx(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "idrx")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Idrx is a free data retrieval call binding the contract method 0xe172c609.
//
// Solidity: function idrx() view returns(address)
func (_AgentTaskManager *AgentTaskManagerSession) Idrx() (common.Address, error) {
	return _AgentTaskManager.Contract.Idrx(&_AgentTaskManager.CallOpts)
}

// Idrx is a free data retrieval call binding the contract method 0xe172c609.
//
// Solidity: function idrx() view returns(address)
func (_AgentTaskManager *AgentTaskManagerCallerSession) Idrx() (common.Address, error) {
	return _AgentTaskManager.Contract.Idrx(&_AgentTaskManager.CallOpts)
}

// IsActive is a free data retrieval call binding the contract method 0x82afd23b.
//
// Solidity: function isActive(uint256 taskId) view returns(bool)
func (_AgentTaskManager *AgentTaskManagerCaller) IsActive(opts *bind.CallOpts, taskId *big.Int) (bool, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "isActive", taskId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsActive is a free data retrieval call binding the contract method 0x82afd23b.
//
// Solidity: function isActive(uint256 taskId) view returns(bool)
func (_AgentTaskManager *AgentTaskManagerSession) IsActive(taskId *big.Int) (bool, error) {
	return _AgentTaskManager.Contract.IsActive(&_AgentTaskManager.CallOpts, taskId)
}

// IsActive is a free data retrieval call binding the contract method 0x82afd23b.
//
// Solidity: function isActive(uint256 taskId) view returns(bool)
func (_AgentTaskManager *AgentTaskManagerCallerSession) IsActive(taskId *big.Int) (bool, error) {
	return _AgentTaskManager.Contract.IsActive(&_AgentTaskManager.CallOpts, taskId)
}

// Protocol is a free data retrieval call binding the contract method 0x8ce74426.
//
// Solidity: function protocol() view returns(address)
func (_AgentTaskManager *AgentTaskManagerCaller) Protocol(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "protocol")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Protocol is a free data retrieval call binding the contract method 0x8ce74426.
//
// Solidity: function protocol() view returns(address)
func (_AgentTaskManager *AgentTaskManagerSession) Protocol() (common.Address, error) {
	return _AgentTaskManager.Contract.Protocol(&_AgentTaskManager.CallOpts)
}

// Protocol is a free data retrieval call binding the contract method 0x8ce74426.
//
// Solidity: function protocol() view returns(address)
func (_AgentTaskManager *AgentTaskManagerCallerSession) Protocol() (common.Address, error) {
	return _AgentTaskManager.Contract.Protocol(&_AgentTaskManager.CallOpts)
}

// SubTaskCount is a free data retrieval call binding the contract method 0x6bea3998.
//
// Solidity: function subTaskCount() view returns(uint256)
func (_AgentTaskManager *AgentTaskManagerCaller) SubTaskCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "subTaskCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// SubTaskCount is a free data retrieval call binding the contract method 0x6bea3998.
//
// Solidity: function subTaskCount() view returns(uint256)
func (_AgentTaskManager *AgentTaskManagerSession) SubTaskCount() (*big.Int, error) {
	return _AgentTaskManager.Contract.SubTaskCount(&_AgentTaskManager.CallOpts)
}

// SubTaskCount is a free data retrieval call binding the contract method 0x6bea3998.
//
// Solidity: function subTaskCount() view returns(uint256)
func (_AgentTaskManager *AgentTaskManagerCallerSession) SubTaskCount() (*big.Int, error) {
	return _AgentTaskManager.Contract.SubTaskCount(&_AgentTaskManager.CallOpts)
}

// SubTaskIdsByTaskId is a free data retrieval call binding the contract method 0x3e77a3bb.
//
// Solidity: function subTaskIdsByTaskId(uint256 taskId, uint256 ) view returns(uint256 subTaskIds)
func (_AgentTaskManager *AgentTaskManagerCaller) SubTaskIdsByTaskId(opts *bind.CallOpts, taskId *big.Int, arg1 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "subTaskIdsByTaskId", taskId, arg1)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// SubTaskIdsByTaskId is a free data retrieval call binding the contract method 0x3e77a3bb.
//
// Solidity: function subTaskIdsByTaskId(uint256 taskId, uint256 ) view returns(uint256 subTaskIds)
func (_AgentTaskManager *AgentTaskManagerSession) SubTaskIdsByTaskId(taskId *big.Int, arg1 *big.Int) (*big.Int, error) {
	return _AgentTaskManager.Contract.SubTaskIdsByTaskId(&_AgentTaskManager.CallOpts, taskId, arg1)
}

// SubTaskIdsByTaskId is a free data retrieval call binding the contract method 0x3e77a3bb.
//
// Solidity: function subTaskIdsByTaskId(uint256 taskId, uint256 ) view returns(uint256 subTaskIds)
func (_AgentTaskManager *AgentTaskManagerCallerSession) SubTaskIdsByTaskId(taskId *big.Int, arg1 *big.Int) (*big.Int, error) {
	return _AgentTaskManager.Contract.SubTaskIdsByTaskId(&_AgentTaskManager.CallOpts, taskId, arg1)
}

// SubTaskIdsForTask is a free data retrieval call binding the contract method 0x96e20e24.
//
// Solidity: function subTaskIdsForTask(uint256 taskId) view returns(uint256[])
func (_AgentTaskManager *AgentTaskManagerCaller) SubTaskIdsForTask(opts *bind.CallOpts, taskId *big.Int) ([]*big.Int, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "subTaskIdsForTask", taskId)

	if err != nil {
		return *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]*big.Int)).(*[]*big.Int)

	return out0, err

}

// SubTaskIdsForTask is a free data retrieval call binding the contract method 0x96e20e24.
//
// Solidity: function subTaskIdsForTask(uint256 taskId) view returns(uint256[])
func (_AgentTaskManager *AgentTaskManagerSession) SubTaskIdsForTask(taskId *big.Int) ([]*big.Int, error) {
	return _AgentTaskManager.Contract.SubTaskIdsForTask(&_AgentTaskManager.CallOpts, taskId)
}

// SubTaskIdsForTask is a free data retrieval call binding the contract method 0x96e20e24.
//
// Solidity: function subTaskIdsForTask(uint256 taskId) view returns(uint256[])
func (_AgentTaskManager *AgentTaskManagerCallerSession) SubTaskIdsForTask(taskId *big.Int) ([]*big.Int, error) {
	return _AgentTaskManager.Contract.SubTaskIdsForTask(&_AgentTaskManager.CallOpts, taskId)
}

// SubTasks is a free data retrieval call binding the contract method 0x7e64a297.
//
// Solidity: function subTasks(uint256 subTaskId) view returns(uint256 taskId, uint8 agent, string stepName, uint8 status, string routingTarget, string summary, bytes32 reasoningHash, bytes32 outputHash, bytes32 decisionHash, bytes32 previousDecisionHash)
func (_AgentTaskManager *AgentTaskManagerCaller) SubTasks(opts *bind.CallOpts, subTaskId *big.Int) (struct {
	TaskId               *big.Int
	Agent                uint8
	StepName             string
	Status               uint8
	RoutingTarget        string
	Summary              string
	ReasoningHash        [32]byte
	OutputHash           [32]byte
	DecisionHash         [32]byte
	PreviousDecisionHash [32]byte
}, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "subTasks", subTaskId)

	outstruct := new(struct {
		TaskId               *big.Int
		Agent                uint8
		StepName             string
		Status               uint8
		RoutingTarget        string
		Summary              string
		ReasoningHash        [32]byte
		OutputHash           [32]byte
		DecisionHash         [32]byte
		PreviousDecisionHash [32]byte
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.TaskId = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.Agent = *abi.ConvertType(out[1], new(uint8)).(*uint8)
	outstruct.StepName = *abi.ConvertType(out[2], new(string)).(*string)
	outstruct.Status = *abi.ConvertType(out[3], new(uint8)).(*uint8)
	outstruct.RoutingTarget = *abi.ConvertType(out[4], new(string)).(*string)
	outstruct.Summary = *abi.ConvertType(out[5], new(string)).(*string)
	outstruct.ReasoningHash = *abi.ConvertType(out[6], new([32]byte)).(*[32]byte)
	outstruct.OutputHash = *abi.ConvertType(out[7], new([32]byte)).(*[32]byte)
	outstruct.DecisionHash = *abi.ConvertType(out[8], new([32]byte)).(*[32]byte)
	outstruct.PreviousDecisionHash = *abi.ConvertType(out[9], new([32]byte)).(*[32]byte)

	return *outstruct, err

}

// SubTasks is a free data retrieval call binding the contract method 0x7e64a297.
//
// Solidity: function subTasks(uint256 subTaskId) view returns(uint256 taskId, uint8 agent, string stepName, uint8 status, string routingTarget, string summary, bytes32 reasoningHash, bytes32 outputHash, bytes32 decisionHash, bytes32 previousDecisionHash)
func (_AgentTaskManager *AgentTaskManagerSession) SubTasks(subTaskId *big.Int) (struct {
	TaskId               *big.Int
	Agent                uint8
	StepName             string
	Status               uint8
	RoutingTarget        string
	Summary              string
	ReasoningHash        [32]byte
	OutputHash           [32]byte
	DecisionHash         [32]byte
	PreviousDecisionHash [32]byte
}, error) {
	return _AgentTaskManager.Contract.SubTasks(&_AgentTaskManager.CallOpts, subTaskId)
}

// SubTasks is a free data retrieval call binding the contract method 0x7e64a297.
//
// Solidity: function subTasks(uint256 subTaskId) view returns(uint256 taskId, uint8 agent, string stepName, uint8 status, string routingTarget, string summary, bytes32 reasoningHash, bytes32 outputHash, bytes32 decisionHash, bytes32 previousDecisionHash)
func (_AgentTaskManager *AgentTaskManagerCallerSession) SubTasks(subTaskId *big.Int) (struct {
	TaskId               *big.Int
	Agent                uint8
	StepName             string
	Status               uint8
	RoutingTarget        string
	Summary              string
	ReasoningHash        [32]byte
	OutputHash           [32]byte
	DecisionHash         [32]byte
	PreviousDecisionHash [32]byte
}, error) {
	return _AgentTaskManager.Contract.SubTasks(&_AgentTaskManager.CallOpts, subTaskId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_AgentTaskManager *AgentTaskManagerCaller) SupportsInterface(opts *bind.CallOpts, interfaceId [4]byte) (bool, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "supportsInterface", interfaceId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_AgentTaskManager *AgentTaskManagerSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _AgentTaskManager.Contract.SupportsInterface(&_AgentTaskManager.CallOpts, interfaceId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_AgentTaskManager *AgentTaskManagerCallerSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _AgentTaskManager.Contract.SupportsInterface(&_AgentTaskManager.CallOpts, interfaceId)
}

// TaskCount is a free data retrieval call binding the contract method 0xb6cb58a5.
//
// Solidity: function taskCount() view returns(uint256)
func (_AgentTaskManager *AgentTaskManagerCaller) TaskCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "taskCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TaskCount is a free data retrieval call binding the contract method 0xb6cb58a5.
//
// Solidity: function taskCount() view returns(uint256)
func (_AgentTaskManager *AgentTaskManagerSession) TaskCount() (*big.Int, error) {
	return _AgentTaskManager.Contract.TaskCount(&_AgentTaskManager.CallOpts)
}

// TaskCount is a free data retrieval call binding the contract method 0xb6cb58a5.
//
// Solidity: function taskCount() view returns(uint256)
func (_AgentTaskManager *AgentTaskManagerCallerSession) TaskCount() (*big.Int, error) {
	return _AgentTaskManager.Contract.TaskCount(&_AgentTaskManager.CallOpts)
}

// Tasks is a free data retrieval call binding the contract method 0x8d977672.
//
// Solidity: function tasks(uint256 taskId) view returns(address owner, bool isActionable, uint8 status, string summary, bytes32 promptHash)
func (_AgentTaskManager *AgentTaskManagerCaller) Tasks(opts *bind.CallOpts, taskId *big.Int) (struct {
	Owner        common.Address
	IsActionable bool
	Status       uint8
	Summary      string
	PromptHash   [32]byte
}, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "tasks", taskId)

	outstruct := new(struct {
		Owner        common.Address
		IsActionable bool
		Status       uint8
		Summary      string
		PromptHash   [32]byte
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Owner = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.IsActionable = *abi.ConvertType(out[1], new(bool)).(*bool)
	outstruct.Status = *abi.ConvertType(out[2], new(uint8)).(*uint8)
	outstruct.Summary = *abi.ConvertType(out[3], new(string)).(*string)
	outstruct.PromptHash = *abi.ConvertType(out[4], new([32]byte)).(*[32]byte)

	return *outstruct, err

}

// Tasks is a free data retrieval call binding the contract method 0x8d977672.
//
// Solidity: function tasks(uint256 taskId) view returns(address owner, bool isActionable, uint8 status, string summary, bytes32 promptHash)
func (_AgentTaskManager *AgentTaskManagerSession) Tasks(taskId *big.Int) (struct {
	Owner        common.Address
	IsActionable bool
	Status       uint8
	Summary      string
	PromptHash   [32]byte
}, error) {
	return _AgentTaskManager.Contract.Tasks(&_AgentTaskManager.CallOpts, taskId)
}

// Tasks is a free data retrieval call binding the contract method 0x8d977672.
//
// Solidity: function tasks(uint256 taskId) view returns(address owner, bool isActionable, uint8 status, string summary, bytes32 promptHash)
func (_AgentTaskManager *AgentTaskManagerCallerSession) Tasks(taskId *big.Int) (struct {
	Owner        common.Address
	IsActionable bool
	Status       uint8
	Summary      string
	PromptHash   [32]byte
}, error) {
	return _AgentTaskManager.Contract.Tasks(&_AgentTaskManager.CallOpts, taskId)
}

// TradeCount is a free data retrieval call binding the contract method 0xbd55022a.
//
// Solidity: function tradeCount() view returns(uint256)
func (_AgentTaskManager *AgentTaskManagerCaller) TradeCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "tradeCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TradeCount is a free data retrieval call binding the contract method 0xbd55022a.
//
// Solidity: function tradeCount() view returns(uint256)
func (_AgentTaskManager *AgentTaskManagerSession) TradeCount() (*big.Int, error) {
	return _AgentTaskManager.Contract.TradeCount(&_AgentTaskManager.CallOpts)
}

// TradeCount is a free data retrieval call binding the contract method 0xbd55022a.
//
// Solidity: function tradeCount() view returns(uint256)
func (_AgentTaskManager *AgentTaskManagerCallerSession) TradeCount() (*big.Int, error) {
	return _AgentTaskManager.Contract.TradeCount(&_AgentTaskManager.CallOpts)
}

// TradeIdsByTaskId is a free data retrieval call binding the contract method 0xd53fba40.
//
// Solidity: function tradeIdsByTaskId(uint256 taskId, uint256 ) view returns(uint256 tradeIds)
func (_AgentTaskManager *AgentTaskManagerCaller) TradeIdsByTaskId(opts *bind.CallOpts, taskId *big.Int, arg1 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "tradeIdsByTaskId", taskId, arg1)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TradeIdsByTaskId is a free data retrieval call binding the contract method 0xd53fba40.
//
// Solidity: function tradeIdsByTaskId(uint256 taskId, uint256 ) view returns(uint256 tradeIds)
func (_AgentTaskManager *AgentTaskManagerSession) TradeIdsByTaskId(taskId *big.Int, arg1 *big.Int) (*big.Int, error) {
	return _AgentTaskManager.Contract.TradeIdsByTaskId(&_AgentTaskManager.CallOpts, taskId, arg1)
}

// TradeIdsByTaskId is a free data retrieval call binding the contract method 0xd53fba40.
//
// Solidity: function tradeIdsByTaskId(uint256 taskId, uint256 ) view returns(uint256 tradeIds)
func (_AgentTaskManager *AgentTaskManagerCallerSession) TradeIdsByTaskId(taskId *big.Int, arg1 *big.Int) (*big.Int, error) {
	return _AgentTaskManager.Contract.TradeIdsByTaskId(&_AgentTaskManager.CallOpts, taskId, arg1)
}

// TradeIdsForTask is a free data retrieval call binding the contract method 0x5b455920.
//
// Solidity: function tradeIdsForTask(uint256 taskId) view returns(uint256[])
func (_AgentTaskManager *AgentTaskManagerCaller) TradeIdsForTask(opts *bind.CallOpts, taskId *big.Int) ([]*big.Int, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "tradeIdsForTask", taskId)

	if err != nil {
		return *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]*big.Int)).(*[]*big.Int)

	return out0, err

}

// TradeIdsForTask is a free data retrieval call binding the contract method 0x5b455920.
//
// Solidity: function tradeIdsForTask(uint256 taskId) view returns(uint256[])
func (_AgentTaskManager *AgentTaskManagerSession) TradeIdsForTask(taskId *big.Int) ([]*big.Int, error) {
	return _AgentTaskManager.Contract.TradeIdsForTask(&_AgentTaskManager.CallOpts, taskId)
}

// TradeIdsForTask is a free data retrieval call binding the contract method 0x5b455920.
//
// Solidity: function tradeIdsForTask(uint256 taskId) view returns(uint256[])
func (_AgentTaskManager *AgentTaskManagerCallerSession) TradeIdsForTask(taskId *big.Int) ([]*big.Int, error) {
	return _AgentTaskManager.Contract.TradeIdsForTask(&_AgentTaskManager.CallOpts, taskId)
}

// TradePermissions is a free data retrieval call binding the contract method 0x3b4e23ab.
//
// Solidity: function tradePermissions(uint256 taskId) view returns(uint256 totalBudget, uint256 usedBudget, uint64 expiresAt)
func (_AgentTaskManager *AgentTaskManagerCaller) TradePermissions(opts *bind.CallOpts, taskId *big.Int) (struct {
	TotalBudget *big.Int
	UsedBudget  *big.Int
	ExpiresAt   uint64
}, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "tradePermissions", taskId)

	outstruct := new(struct {
		TotalBudget *big.Int
		UsedBudget  *big.Int
		ExpiresAt   uint64
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.TotalBudget = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.UsedBudget = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.ExpiresAt = *abi.ConvertType(out[2], new(uint64)).(*uint64)

	return *outstruct, err

}

// TradePermissions is a free data retrieval call binding the contract method 0x3b4e23ab.
//
// Solidity: function tradePermissions(uint256 taskId) view returns(uint256 totalBudget, uint256 usedBudget, uint64 expiresAt)
func (_AgentTaskManager *AgentTaskManagerSession) TradePermissions(taskId *big.Int) (struct {
	TotalBudget *big.Int
	UsedBudget  *big.Int
	ExpiresAt   uint64
}, error) {
	return _AgentTaskManager.Contract.TradePermissions(&_AgentTaskManager.CallOpts, taskId)
}

// TradePermissions is a free data retrieval call binding the contract method 0x3b4e23ab.
//
// Solidity: function tradePermissions(uint256 taskId) view returns(uint256 totalBudget, uint256 usedBudget, uint64 expiresAt)
func (_AgentTaskManager *AgentTaskManagerCallerSession) TradePermissions(taskId *big.Int) (struct {
	TotalBudget *big.Int
	UsedBudget  *big.Int
	ExpiresAt   uint64
}, error) {
	return _AgentTaskManager.Contract.TradePermissions(&_AgentTaskManager.CallOpts, taskId)
}

// Trades is a free data retrieval call binding the contract method 0x1e6c598e.
//
// Solidity: function trades(uint256 tradeId) view returns(uint256 taskId, uint256 subTaskId, string ticker, uint8 side, uint256 amount, uint256 receivedAmount, string summary, bytes32 reasoningHash, uint256 executedAt)
func (_AgentTaskManager *AgentTaskManagerCaller) Trades(opts *bind.CallOpts, tradeId *big.Int) (struct {
	TaskId         *big.Int
	SubTaskId      *big.Int
	Ticker         string
	Side           uint8
	Amount         *big.Int
	ReceivedAmount *big.Int
	Summary        string
	ReasoningHash  [32]byte
	ExecutedAt     *big.Int
}, error) {
	var out []interface{}
	err := _AgentTaskManager.contract.Call(opts, &out, "trades", tradeId)

	outstruct := new(struct {
		TaskId         *big.Int
		SubTaskId      *big.Int
		Ticker         string
		Side           uint8
		Amount         *big.Int
		ReceivedAmount *big.Int
		Summary        string
		ReasoningHash  [32]byte
		ExecutedAt     *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.TaskId = *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)
	outstruct.SubTaskId = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.Ticker = *abi.ConvertType(out[2], new(string)).(*string)
	outstruct.Side = *abi.ConvertType(out[3], new(uint8)).(*uint8)
	outstruct.Amount = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)
	outstruct.ReceivedAmount = *abi.ConvertType(out[5], new(*big.Int)).(**big.Int)
	outstruct.Summary = *abi.ConvertType(out[6], new(string)).(*string)
	outstruct.ReasoningHash = *abi.ConvertType(out[7], new([32]byte)).(*[32]byte)
	outstruct.ExecutedAt = *abi.ConvertType(out[8], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// Trades is a free data retrieval call binding the contract method 0x1e6c598e.
//
// Solidity: function trades(uint256 tradeId) view returns(uint256 taskId, uint256 subTaskId, string ticker, uint8 side, uint256 amount, uint256 receivedAmount, string summary, bytes32 reasoningHash, uint256 executedAt)
func (_AgentTaskManager *AgentTaskManagerSession) Trades(tradeId *big.Int) (struct {
	TaskId         *big.Int
	SubTaskId      *big.Int
	Ticker         string
	Side           uint8
	Amount         *big.Int
	ReceivedAmount *big.Int
	Summary        string
	ReasoningHash  [32]byte
	ExecutedAt     *big.Int
}, error) {
	return _AgentTaskManager.Contract.Trades(&_AgentTaskManager.CallOpts, tradeId)
}

// Trades is a free data retrieval call binding the contract method 0x1e6c598e.
//
// Solidity: function trades(uint256 tradeId) view returns(uint256 taskId, uint256 subTaskId, string ticker, uint8 side, uint256 amount, uint256 receivedAmount, string summary, bytes32 reasoningHash, uint256 executedAt)
func (_AgentTaskManager *AgentTaskManagerCallerSession) Trades(tradeId *big.Int) (struct {
	TaskId         *big.Int
	SubTaskId      *big.Int
	Ticker         string
	Side           uint8
	Amount         *big.Int
	ReceivedAmount *big.Int
	Summary        string
	ReasoningHash  [32]byte
	ExecutedAt     *big.Int
}, error) {
	return _AgentTaskManager.Contract.Trades(&_AgentTaskManager.CallOpts, tradeId)
}

// CancelTask is a paid mutator transaction binding the contract method 0x7eec20a8.
//
// Solidity: function cancelTask(uint256 taskId) returns()
func (_AgentTaskManager *AgentTaskManagerTransactor) CancelTask(opts *bind.TransactOpts, taskId *big.Int) (*types.Transaction, error) {
	return _AgentTaskManager.contract.Transact(opts, "cancelTask", taskId)
}

// CancelTask is a paid mutator transaction binding the contract method 0x7eec20a8.
//
// Solidity: function cancelTask(uint256 taskId) returns()
func (_AgentTaskManager *AgentTaskManagerSession) CancelTask(taskId *big.Int) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.CancelTask(&_AgentTaskManager.TransactOpts, taskId)
}

// CancelTask is a paid mutator transaction binding the contract method 0x7eec20a8.
//
// Solidity: function cancelTask(uint256 taskId) returns()
func (_AgentTaskManager *AgentTaskManagerTransactorSession) CancelTask(taskId *big.Int) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.CancelTask(&_AgentTaskManager.TransactOpts, taskId)
}

// CreateTask is a paid mutator transaction binding the contract method 0x6fc599b3.
//
// Solidity: function createTask(address owner, bool isActionable, string summary, bytes32 promptHash) returns(uint256 taskId)
func (_AgentTaskManager *AgentTaskManagerTransactor) CreateTask(opts *bind.TransactOpts, owner common.Address, isActionable bool, summary string, promptHash [32]byte) (*types.Transaction, error) {
	return _AgentTaskManager.contract.Transact(opts, "createTask", owner, isActionable, summary, promptHash)
}

// CreateTask is a paid mutator transaction binding the contract method 0x6fc599b3.
//
// Solidity: function createTask(address owner, bool isActionable, string summary, bytes32 promptHash) returns(uint256 taskId)
func (_AgentTaskManager *AgentTaskManagerSession) CreateTask(owner common.Address, isActionable bool, summary string, promptHash [32]byte) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.CreateTask(&_AgentTaskManager.TransactOpts, owner, isActionable, summary, promptHash)
}

// CreateTask is a paid mutator transaction binding the contract method 0x6fc599b3.
//
// Solidity: function createTask(address owner, bool isActionable, string summary, bytes32 promptHash) returns(uint256 taskId)
func (_AgentTaskManager *AgentTaskManagerTransactorSession) CreateTask(owner common.Address, isActionable bool, summary string, promptHash [32]byte) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.CreateTask(&_AgentTaskManager.TransactOpts, owner, isActionable, summary, promptHash)
}

// ExecuteTrade is a paid mutator transaction binding the contract method 0x575c7e50.
//
// Solidity: function executeTrade(uint256 taskId, uint256 subTaskId, address token, string ticker, uint8 side, uint256 amount, uint256 minimumOutputAmount, string summary, bytes32 reasoningHash) returns(uint256 tradeId)
func (_AgentTaskManager *AgentTaskManagerTransactor) ExecuteTrade(opts *bind.TransactOpts, taskId *big.Int, subTaskId *big.Int, token common.Address, ticker string, side uint8, amount *big.Int, minimumOutputAmount *big.Int, summary string, reasoningHash [32]byte) (*types.Transaction, error) {
	return _AgentTaskManager.contract.Transact(opts, "executeTrade", taskId, subTaskId, token, ticker, side, amount, minimumOutputAmount, summary, reasoningHash)
}

// ExecuteTrade is a paid mutator transaction binding the contract method 0x575c7e50.
//
// Solidity: function executeTrade(uint256 taskId, uint256 subTaskId, address token, string ticker, uint8 side, uint256 amount, uint256 minimumOutputAmount, string summary, bytes32 reasoningHash) returns(uint256 tradeId)
func (_AgentTaskManager *AgentTaskManagerSession) ExecuteTrade(taskId *big.Int, subTaskId *big.Int, token common.Address, ticker string, side uint8, amount *big.Int, minimumOutputAmount *big.Int, summary string, reasoningHash [32]byte) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.ExecuteTrade(&_AgentTaskManager.TransactOpts, taskId, subTaskId, token, ticker, side, amount, minimumOutputAmount, summary, reasoningHash)
}

// ExecuteTrade is a paid mutator transaction binding the contract method 0x575c7e50.
//
// Solidity: function executeTrade(uint256 taskId, uint256 subTaskId, address token, string ticker, uint8 side, uint256 amount, uint256 minimumOutputAmount, string summary, bytes32 reasoningHash) returns(uint256 tradeId)
func (_AgentTaskManager *AgentTaskManagerTransactorSession) ExecuteTrade(taskId *big.Int, subTaskId *big.Int, token common.Address, ticker string, side uint8, amount *big.Int, minimumOutputAmount *big.Int, summary string, reasoningHash [32]byte) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.ExecuteTrade(&_AgentTaskManager.TransactOpts, taskId, subTaskId, token, ticker, side, amount, minimumOutputAmount, summary, reasoningHash)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_AgentTaskManager *AgentTaskManagerTransactor) GrantRole(opts *bind.TransactOpts, role [32]byte, account common.Address) (*types.Transaction, error) {
	return _AgentTaskManager.contract.Transact(opts, "grantRole", role, account)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_AgentTaskManager *AgentTaskManagerSession) GrantRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.GrantRole(&_AgentTaskManager.TransactOpts, role, account)
}

// GrantRole is a paid mutator transaction binding the contract method 0x2f2ff15d.
//
// Solidity: function grantRole(bytes32 role, address account) returns()
func (_AgentTaskManager *AgentTaskManagerTransactorSession) GrantRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.GrantRole(&_AgentTaskManager.TransactOpts, role, account)
}

// GrantTradePermission is a paid mutator transaction binding the contract method 0xa80edf02.
//
// Solidity: function grantTradePermission(uint256 taskId, uint256 totalBudget, uint256 duration) returns()
func (_AgentTaskManager *AgentTaskManagerTransactor) GrantTradePermission(opts *bind.TransactOpts, taskId *big.Int, totalBudget *big.Int, duration *big.Int) (*types.Transaction, error) {
	return _AgentTaskManager.contract.Transact(opts, "grantTradePermission", taskId, totalBudget, duration)
}

// GrantTradePermission is a paid mutator transaction binding the contract method 0xa80edf02.
//
// Solidity: function grantTradePermission(uint256 taskId, uint256 totalBudget, uint256 duration) returns()
func (_AgentTaskManager *AgentTaskManagerSession) GrantTradePermission(taskId *big.Int, totalBudget *big.Int, duration *big.Int) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.GrantTradePermission(&_AgentTaskManager.TransactOpts, taskId, totalBudget, duration)
}

// GrantTradePermission is a paid mutator transaction binding the contract method 0xa80edf02.
//
// Solidity: function grantTradePermission(uint256 taskId, uint256 totalBudget, uint256 duration) returns()
func (_AgentTaskManager *AgentTaskManagerTransactorSession) GrantTradePermission(taskId *big.Int, totalBudget *big.Int, duration *big.Int) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.GrantTradePermission(&_AgentTaskManager.TransactOpts, taskId, totalBudget, duration)
}

// RecordSubTasks is a paid mutator transaction binding the contract method 0xd4243b1a.
//
// Solidity: function recordSubTasks(uint256 taskId, (uint256,uint8,string,uint8,string,string,bytes32,bytes32,bytes32,bytes32)[] subTaskRecords) returns(uint256[] subTaskIds)
func (_AgentTaskManager *AgentTaskManagerTransactor) RecordSubTasks(opts *bind.TransactOpts, taskId *big.Int, subTaskRecords []AgentTaskManagerSubTaskRecord) (*types.Transaction, error) {
	return _AgentTaskManager.contract.Transact(opts, "recordSubTasks", taskId, subTaskRecords)
}

// RecordSubTasks is a paid mutator transaction binding the contract method 0xd4243b1a.
//
// Solidity: function recordSubTasks(uint256 taskId, (uint256,uint8,string,uint8,string,string,bytes32,bytes32,bytes32,bytes32)[] subTaskRecords) returns(uint256[] subTaskIds)
func (_AgentTaskManager *AgentTaskManagerSession) RecordSubTasks(taskId *big.Int, subTaskRecords []AgentTaskManagerSubTaskRecord) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.RecordSubTasks(&_AgentTaskManager.TransactOpts, taskId, subTaskRecords)
}

// RecordSubTasks is a paid mutator transaction binding the contract method 0xd4243b1a.
//
// Solidity: function recordSubTasks(uint256 taskId, (uint256,uint8,string,uint8,string,string,bytes32,bytes32,bytes32,bytes32)[] subTaskRecords) returns(uint256[] subTaskIds)
func (_AgentTaskManager *AgentTaskManagerTransactorSession) RecordSubTasks(taskId *big.Int, subTaskRecords []AgentTaskManagerSubTaskRecord) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.RecordSubTasks(&_AgentTaskManager.TransactOpts, taskId, subTaskRecords)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (_AgentTaskManager *AgentTaskManagerTransactor) RenounceRole(opts *bind.TransactOpts, role [32]byte, callerConfirmation common.Address) (*types.Transaction, error) {
	return _AgentTaskManager.contract.Transact(opts, "renounceRole", role, callerConfirmation)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (_AgentTaskManager *AgentTaskManagerSession) RenounceRole(role [32]byte, callerConfirmation common.Address) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.RenounceRole(&_AgentTaskManager.TransactOpts, role, callerConfirmation)
}

// RenounceRole is a paid mutator transaction binding the contract method 0x36568abe.
//
// Solidity: function renounceRole(bytes32 role, address callerConfirmation) returns()
func (_AgentTaskManager *AgentTaskManagerTransactorSession) RenounceRole(role [32]byte, callerConfirmation common.Address) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.RenounceRole(&_AgentTaskManager.TransactOpts, role, callerConfirmation)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_AgentTaskManager *AgentTaskManagerTransactor) RevokeRole(opts *bind.TransactOpts, role [32]byte, account common.Address) (*types.Transaction, error) {
	return _AgentTaskManager.contract.Transact(opts, "revokeRole", role, account)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_AgentTaskManager *AgentTaskManagerSession) RevokeRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.RevokeRole(&_AgentTaskManager.TransactOpts, role, account)
}

// RevokeRole is a paid mutator transaction binding the contract method 0xd547741f.
//
// Solidity: function revokeRole(bytes32 role, address account) returns()
func (_AgentTaskManager *AgentTaskManagerTransactorSession) RevokeRole(role [32]byte, account common.Address) (*types.Transaction, error) {
	return _AgentTaskManager.Contract.RevokeRole(&_AgentTaskManager.TransactOpts, role, account)
}

// AgentTaskManagerRoleAdminChangedIterator is returned from FilterRoleAdminChanged and is used to iterate over the raw logs and unpacked data for RoleAdminChanged events raised by the AgentTaskManager contract.
type AgentTaskManagerRoleAdminChangedIterator struct {
	Event *AgentTaskManagerRoleAdminChanged // Event containing the contract specifics and raw log

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
func (it *AgentTaskManagerRoleAdminChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AgentTaskManagerRoleAdminChanged)
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
		it.Event = new(AgentTaskManagerRoleAdminChanged)
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
func (it *AgentTaskManagerRoleAdminChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AgentTaskManagerRoleAdminChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AgentTaskManagerRoleAdminChanged represents a RoleAdminChanged event raised by the AgentTaskManager contract.
type AgentTaskManagerRoleAdminChanged struct {
	Role              [32]byte
	PreviousAdminRole [32]byte
	NewAdminRole      [32]byte
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterRoleAdminChanged is a free log retrieval operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_AgentTaskManager *AgentTaskManagerFilterer) FilterRoleAdminChanged(opts *bind.FilterOpts, role [][32]byte, previousAdminRole [][32]byte, newAdminRole [][32]byte) (*AgentTaskManagerRoleAdminChangedIterator, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var previousAdminRoleRule []interface{}
	for _, previousAdminRoleItem := range previousAdminRole {
		previousAdminRoleRule = append(previousAdminRoleRule, previousAdminRoleItem)
	}
	var newAdminRoleRule []interface{}
	for _, newAdminRoleItem := range newAdminRole {
		newAdminRoleRule = append(newAdminRoleRule, newAdminRoleItem)
	}

	logs, sub, err := _AgentTaskManager.contract.FilterLogs(opts, "RoleAdminChanged", roleRule, previousAdminRoleRule, newAdminRoleRule)
	if err != nil {
		return nil, err
	}
	return &AgentTaskManagerRoleAdminChangedIterator{contract: _AgentTaskManager.contract, event: "RoleAdminChanged", logs: logs, sub: sub}, nil
}

// WatchRoleAdminChanged is a free log subscription operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_AgentTaskManager *AgentTaskManagerFilterer) WatchRoleAdminChanged(opts *bind.WatchOpts, sink chan<- *AgentTaskManagerRoleAdminChanged, role [][32]byte, previousAdminRole [][32]byte, newAdminRole [][32]byte) (event.Subscription, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var previousAdminRoleRule []interface{}
	for _, previousAdminRoleItem := range previousAdminRole {
		previousAdminRoleRule = append(previousAdminRoleRule, previousAdminRoleItem)
	}
	var newAdminRoleRule []interface{}
	for _, newAdminRoleItem := range newAdminRole {
		newAdminRoleRule = append(newAdminRoleRule, newAdminRoleItem)
	}

	logs, sub, err := _AgentTaskManager.contract.WatchLogs(opts, "RoleAdminChanged", roleRule, previousAdminRoleRule, newAdminRoleRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AgentTaskManagerRoleAdminChanged)
				if err := _AgentTaskManager.contract.UnpackLog(event, "RoleAdminChanged", log); err != nil {
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

// ParseRoleAdminChanged is a log parse operation binding the contract event 0xbd79b86ffe0ab8e8776151514217cd7cacd52c909f66475c3af44e129f0b00ff.
//
// Solidity: event RoleAdminChanged(bytes32 indexed role, bytes32 indexed previousAdminRole, bytes32 indexed newAdminRole)
func (_AgentTaskManager *AgentTaskManagerFilterer) ParseRoleAdminChanged(log types.Log) (*AgentTaskManagerRoleAdminChanged, error) {
	event := new(AgentTaskManagerRoleAdminChanged)
	if err := _AgentTaskManager.contract.UnpackLog(event, "RoleAdminChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AgentTaskManagerRoleGrantedIterator is returned from FilterRoleGranted and is used to iterate over the raw logs and unpacked data for RoleGranted events raised by the AgentTaskManager contract.
type AgentTaskManagerRoleGrantedIterator struct {
	Event *AgentTaskManagerRoleGranted // Event containing the contract specifics and raw log

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
func (it *AgentTaskManagerRoleGrantedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AgentTaskManagerRoleGranted)
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
		it.Event = new(AgentTaskManagerRoleGranted)
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
func (it *AgentTaskManagerRoleGrantedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AgentTaskManagerRoleGrantedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AgentTaskManagerRoleGranted represents a RoleGranted event raised by the AgentTaskManager contract.
type AgentTaskManagerRoleGranted struct {
	Role    [32]byte
	Account common.Address
	Sender  common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRoleGranted is a free log retrieval operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_AgentTaskManager *AgentTaskManagerFilterer) FilterRoleGranted(opts *bind.FilterOpts, role [][32]byte, account []common.Address, sender []common.Address) (*AgentTaskManagerRoleGrantedIterator, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _AgentTaskManager.contract.FilterLogs(opts, "RoleGranted", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &AgentTaskManagerRoleGrantedIterator{contract: _AgentTaskManager.contract, event: "RoleGranted", logs: logs, sub: sub}, nil
}

// WatchRoleGranted is a free log subscription operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_AgentTaskManager *AgentTaskManagerFilterer) WatchRoleGranted(opts *bind.WatchOpts, sink chan<- *AgentTaskManagerRoleGranted, role [][32]byte, account []common.Address, sender []common.Address) (event.Subscription, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _AgentTaskManager.contract.WatchLogs(opts, "RoleGranted", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AgentTaskManagerRoleGranted)
				if err := _AgentTaskManager.contract.UnpackLog(event, "RoleGranted", log); err != nil {
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

// ParseRoleGranted is a log parse operation binding the contract event 0x2f8788117e7eff1d82e926ec794901d17c78024a50270940304540a733656f0d.
//
// Solidity: event RoleGranted(bytes32 indexed role, address indexed account, address indexed sender)
func (_AgentTaskManager *AgentTaskManagerFilterer) ParseRoleGranted(log types.Log) (*AgentTaskManagerRoleGranted, error) {
	event := new(AgentTaskManagerRoleGranted)
	if err := _AgentTaskManager.contract.UnpackLog(event, "RoleGranted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AgentTaskManagerRoleRevokedIterator is returned from FilterRoleRevoked and is used to iterate over the raw logs and unpacked data for RoleRevoked events raised by the AgentTaskManager contract.
type AgentTaskManagerRoleRevokedIterator struct {
	Event *AgentTaskManagerRoleRevoked // Event containing the contract specifics and raw log

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
func (it *AgentTaskManagerRoleRevokedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AgentTaskManagerRoleRevoked)
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
		it.Event = new(AgentTaskManagerRoleRevoked)
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
func (it *AgentTaskManagerRoleRevokedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AgentTaskManagerRoleRevokedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AgentTaskManagerRoleRevoked represents a RoleRevoked event raised by the AgentTaskManager contract.
type AgentTaskManagerRoleRevoked struct {
	Role    [32]byte
	Account common.Address
	Sender  common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterRoleRevoked is a free log retrieval operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_AgentTaskManager *AgentTaskManagerFilterer) FilterRoleRevoked(opts *bind.FilterOpts, role [][32]byte, account []common.Address, sender []common.Address) (*AgentTaskManagerRoleRevokedIterator, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _AgentTaskManager.contract.FilterLogs(opts, "RoleRevoked", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return &AgentTaskManagerRoleRevokedIterator{contract: _AgentTaskManager.contract, event: "RoleRevoked", logs: logs, sub: sub}, nil
}

// WatchRoleRevoked is a free log subscription operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_AgentTaskManager *AgentTaskManagerFilterer) WatchRoleRevoked(opts *bind.WatchOpts, sink chan<- *AgentTaskManagerRoleRevoked, role [][32]byte, account []common.Address, sender []common.Address) (event.Subscription, error) {

	var roleRule []interface{}
	for _, roleItem := range role {
		roleRule = append(roleRule, roleItem)
	}
	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var senderRule []interface{}
	for _, senderItem := range sender {
		senderRule = append(senderRule, senderItem)
	}

	logs, sub, err := _AgentTaskManager.contract.WatchLogs(opts, "RoleRevoked", roleRule, accountRule, senderRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AgentTaskManagerRoleRevoked)
				if err := _AgentTaskManager.contract.UnpackLog(event, "RoleRevoked", log); err != nil {
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

// ParseRoleRevoked is a log parse operation binding the contract event 0xf6391f5c32d9c69d2a47ea670b442974b53935d1edc7fd64eb21e047a839171b.
//
// Solidity: event RoleRevoked(bytes32 indexed role, address indexed account, address indexed sender)
func (_AgentTaskManager *AgentTaskManagerFilterer) ParseRoleRevoked(log types.Log) (*AgentTaskManagerRoleRevoked, error) {
	event := new(AgentTaskManagerRoleRevoked)
	if err := _AgentTaskManager.contract.UnpackLog(event, "RoleRevoked", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AgentTaskManagerSubTaskRecordedIterator is returned from FilterSubTaskRecorded and is used to iterate over the raw logs and unpacked data for SubTaskRecorded events raised by the AgentTaskManager contract.
type AgentTaskManagerSubTaskRecordedIterator struct {
	Event *AgentTaskManagerSubTaskRecorded // Event containing the contract specifics and raw log

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
func (it *AgentTaskManagerSubTaskRecordedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AgentTaskManagerSubTaskRecorded)
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
		it.Event = new(AgentTaskManagerSubTaskRecorded)
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
func (it *AgentTaskManagerSubTaskRecordedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AgentTaskManagerSubTaskRecordedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AgentTaskManagerSubTaskRecorded represents a SubTaskRecorded event raised by the AgentTaskManager contract.
type AgentTaskManagerSubTaskRecorded struct {
	TaskId    *big.Int
	SubTaskId *big.Int
	Agent     uint8
	StepName  string
	Summary   string
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterSubTaskRecorded is a free log retrieval operation binding the contract event 0x0589aaeac5cab8fe2d1361d28e3c202a9f7d06bd4ae5dff69c43a46412e9ed6a.
//
// Solidity: event SubTaskRecorded(uint256 indexed taskId, uint256 subTaskId, uint8 indexed agent, string stepName, string summary)
func (_AgentTaskManager *AgentTaskManagerFilterer) FilterSubTaskRecorded(opts *bind.FilterOpts, taskId []*big.Int, agent []uint8) (*AgentTaskManagerSubTaskRecordedIterator, error) {

	var taskIdRule []interface{}
	for _, taskIdItem := range taskId {
		taskIdRule = append(taskIdRule, taskIdItem)
	}

	var agentRule []interface{}
	for _, agentItem := range agent {
		agentRule = append(agentRule, agentItem)
	}

	logs, sub, err := _AgentTaskManager.contract.FilterLogs(opts, "SubTaskRecorded", taskIdRule, agentRule)
	if err != nil {
		return nil, err
	}
	return &AgentTaskManagerSubTaskRecordedIterator{contract: _AgentTaskManager.contract, event: "SubTaskRecorded", logs: logs, sub: sub}, nil
}

// WatchSubTaskRecorded is a free log subscription operation binding the contract event 0x0589aaeac5cab8fe2d1361d28e3c202a9f7d06bd4ae5dff69c43a46412e9ed6a.
//
// Solidity: event SubTaskRecorded(uint256 indexed taskId, uint256 subTaskId, uint8 indexed agent, string stepName, string summary)
func (_AgentTaskManager *AgentTaskManagerFilterer) WatchSubTaskRecorded(opts *bind.WatchOpts, sink chan<- *AgentTaskManagerSubTaskRecorded, taskId []*big.Int, agent []uint8) (event.Subscription, error) {

	var taskIdRule []interface{}
	for _, taskIdItem := range taskId {
		taskIdRule = append(taskIdRule, taskIdItem)
	}

	var agentRule []interface{}
	for _, agentItem := range agent {
		agentRule = append(agentRule, agentItem)
	}

	logs, sub, err := _AgentTaskManager.contract.WatchLogs(opts, "SubTaskRecorded", taskIdRule, agentRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AgentTaskManagerSubTaskRecorded)
				if err := _AgentTaskManager.contract.UnpackLog(event, "SubTaskRecorded", log); err != nil {
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

// ParseSubTaskRecorded is a log parse operation binding the contract event 0x0589aaeac5cab8fe2d1361d28e3c202a9f7d06bd4ae5dff69c43a46412e9ed6a.
//
// Solidity: event SubTaskRecorded(uint256 indexed taskId, uint256 subTaskId, uint8 indexed agent, string stepName, string summary)
func (_AgentTaskManager *AgentTaskManagerFilterer) ParseSubTaskRecorded(log types.Log) (*AgentTaskManagerSubTaskRecorded, error) {
	event := new(AgentTaskManagerSubTaskRecorded)
	if err := _AgentTaskManager.contract.UnpackLog(event, "SubTaskRecorded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AgentTaskManagerTaskCancelledIterator is returned from FilterTaskCancelled and is used to iterate over the raw logs and unpacked data for TaskCancelled events raised by the AgentTaskManager contract.
type AgentTaskManagerTaskCancelledIterator struct {
	Event *AgentTaskManagerTaskCancelled // Event containing the contract specifics and raw log

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
func (it *AgentTaskManagerTaskCancelledIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AgentTaskManagerTaskCancelled)
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
		it.Event = new(AgentTaskManagerTaskCancelled)
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
func (it *AgentTaskManagerTaskCancelledIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AgentTaskManagerTaskCancelledIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AgentTaskManagerTaskCancelled represents a TaskCancelled event raised by the AgentTaskManager contract.
type AgentTaskManagerTaskCancelled struct {
	TaskId      *big.Int
	CancelledBy common.Address
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterTaskCancelled is a free log retrieval operation binding the contract event 0x76be7f7b12bf790225259ca3b7ffb98f222f79af3c4c074e88e0c97af6f14025.
//
// Solidity: event TaskCancelled(uint256 indexed taskId, address indexed cancelledBy)
func (_AgentTaskManager *AgentTaskManagerFilterer) FilterTaskCancelled(opts *bind.FilterOpts, taskId []*big.Int, cancelledBy []common.Address) (*AgentTaskManagerTaskCancelledIterator, error) {

	var taskIdRule []interface{}
	for _, taskIdItem := range taskId {
		taskIdRule = append(taskIdRule, taskIdItem)
	}
	var cancelledByRule []interface{}
	for _, cancelledByItem := range cancelledBy {
		cancelledByRule = append(cancelledByRule, cancelledByItem)
	}

	logs, sub, err := _AgentTaskManager.contract.FilterLogs(opts, "TaskCancelled", taskIdRule, cancelledByRule)
	if err != nil {
		return nil, err
	}
	return &AgentTaskManagerTaskCancelledIterator{contract: _AgentTaskManager.contract, event: "TaskCancelled", logs: logs, sub: sub}, nil
}

// WatchTaskCancelled is a free log subscription operation binding the contract event 0x76be7f7b12bf790225259ca3b7ffb98f222f79af3c4c074e88e0c97af6f14025.
//
// Solidity: event TaskCancelled(uint256 indexed taskId, address indexed cancelledBy)
func (_AgentTaskManager *AgentTaskManagerFilterer) WatchTaskCancelled(opts *bind.WatchOpts, sink chan<- *AgentTaskManagerTaskCancelled, taskId []*big.Int, cancelledBy []common.Address) (event.Subscription, error) {

	var taskIdRule []interface{}
	for _, taskIdItem := range taskId {
		taskIdRule = append(taskIdRule, taskIdItem)
	}
	var cancelledByRule []interface{}
	for _, cancelledByItem := range cancelledBy {
		cancelledByRule = append(cancelledByRule, cancelledByItem)
	}

	logs, sub, err := _AgentTaskManager.contract.WatchLogs(opts, "TaskCancelled", taskIdRule, cancelledByRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AgentTaskManagerTaskCancelled)
				if err := _AgentTaskManager.contract.UnpackLog(event, "TaskCancelled", log); err != nil {
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

// ParseTaskCancelled is a log parse operation binding the contract event 0x76be7f7b12bf790225259ca3b7ffb98f222f79af3c4c074e88e0c97af6f14025.
//
// Solidity: event TaskCancelled(uint256 indexed taskId, address indexed cancelledBy)
func (_AgentTaskManager *AgentTaskManagerFilterer) ParseTaskCancelled(log types.Log) (*AgentTaskManagerTaskCancelled, error) {
	event := new(AgentTaskManagerTaskCancelled)
	if err := _AgentTaskManager.contract.UnpackLog(event, "TaskCancelled", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AgentTaskManagerTaskCreatedIterator is returned from FilterTaskCreated and is used to iterate over the raw logs and unpacked data for TaskCreated events raised by the AgentTaskManager contract.
type AgentTaskManagerTaskCreatedIterator struct {
	Event *AgentTaskManagerTaskCreated // Event containing the contract specifics and raw log

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
func (it *AgentTaskManagerTaskCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AgentTaskManagerTaskCreated)
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
		it.Event = new(AgentTaskManagerTaskCreated)
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
func (it *AgentTaskManagerTaskCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AgentTaskManagerTaskCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AgentTaskManagerTaskCreated represents a TaskCreated event raised by the AgentTaskManager contract.
type AgentTaskManagerTaskCreated struct {
	TaskId       *big.Int
	Owner        common.Address
	IsActionable bool
	Summary      string
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterTaskCreated is a free log retrieval operation binding the contract event 0x8d7331c188be27e20f5b25d6b4fcbedc0de3b7409d029561512cc81445e7576c.
//
// Solidity: event TaskCreated(uint256 indexed taskId, address indexed owner, bool isActionable, string summary)
func (_AgentTaskManager *AgentTaskManagerFilterer) FilterTaskCreated(opts *bind.FilterOpts, taskId []*big.Int, owner []common.Address) (*AgentTaskManagerTaskCreatedIterator, error) {

	var taskIdRule []interface{}
	for _, taskIdItem := range taskId {
		taskIdRule = append(taskIdRule, taskIdItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _AgentTaskManager.contract.FilterLogs(opts, "TaskCreated", taskIdRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return &AgentTaskManagerTaskCreatedIterator{contract: _AgentTaskManager.contract, event: "TaskCreated", logs: logs, sub: sub}, nil
}

// WatchTaskCreated is a free log subscription operation binding the contract event 0x8d7331c188be27e20f5b25d6b4fcbedc0de3b7409d029561512cc81445e7576c.
//
// Solidity: event TaskCreated(uint256 indexed taskId, address indexed owner, bool isActionable, string summary)
func (_AgentTaskManager *AgentTaskManagerFilterer) WatchTaskCreated(opts *bind.WatchOpts, sink chan<- *AgentTaskManagerTaskCreated, taskId []*big.Int, owner []common.Address) (event.Subscription, error) {

	var taskIdRule []interface{}
	for _, taskIdItem := range taskId {
		taskIdRule = append(taskIdRule, taskIdItem)
	}
	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _AgentTaskManager.contract.WatchLogs(opts, "TaskCreated", taskIdRule, ownerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AgentTaskManagerTaskCreated)
				if err := _AgentTaskManager.contract.UnpackLog(event, "TaskCreated", log); err != nil {
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

// ParseTaskCreated is a log parse operation binding the contract event 0x8d7331c188be27e20f5b25d6b4fcbedc0de3b7409d029561512cc81445e7576c.
//
// Solidity: event TaskCreated(uint256 indexed taskId, address indexed owner, bool isActionable, string summary)
func (_AgentTaskManager *AgentTaskManagerFilterer) ParseTaskCreated(log types.Log) (*AgentTaskManagerTaskCreated, error) {
	event := new(AgentTaskManagerTaskCreated)
	if err := _AgentTaskManager.contract.UnpackLog(event, "TaskCreated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AgentTaskManagerTradeExecutedIterator is returned from FilterTradeExecuted and is used to iterate over the raw logs and unpacked data for TradeExecuted events raised by the AgentTaskManager contract.
type AgentTaskManagerTradeExecutedIterator struct {
	Event *AgentTaskManagerTradeExecuted // Event containing the contract specifics and raw log

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
func (it *AgentTaskManagerTradeExecutedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AgentTaskManagerTradeExecuted)
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
		it.Event = new(AgentTaskManagerTradeExecuted)
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
func (it *AgentTaskManagerTradeExecutedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AgentTaskManagerTradeExecutedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AgentTaskManagerTradeExecuted represents a TradeExecuted event raised by the AgentTaskManager contract.
type AgentTaskManagerTradeExecuted struct {
	TaskId  *big.Int
	TradeId *big.Int
	Ticker  string
	Side    uint8
	Amount  *big.Int
	Summary string
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterTradeExecuted is a free log retrieval operation binding the contract event 0x0dff0d471c9d498b59146ae96d69adade0b56906aab4f3c1a33b450d88b083e9.
//
// Solidity: event TradeExecuted(uint256 indexed taskId, uint256 indexed tradeId, string ticker, uint8 side, uint256 amount, string summary)
func (_AgentTaskManager *AgentTaskManagerFilterer) FilterTradeExecuted(opts *bind.FilterOpts, taskId []*big.Int, tradeId []*big.Int) (*AgentTaskManagerTradeExecutedIterator, error) {

	var taskIdRule []interface{}
	for _, taskIdItem := range taskId {
		taskIdRule = append(taskIdRule, taskIdItem)
	}
	var tradeIdRule []interface{}
	for _, tradeIdItem := range tradeId {
		tradeIdRule = append(tradeIdRule, tradeIdItem)
	}

	logs, sub, err := _AgentTaskManager.contract.FilterLogs(opts, "TradeExecuted", taskIdRule, tradeIdRule)
	if err != nil {
		return nil, err
	}
	return &AgentTaskManagerTradeExecutedIterator{contract: _AgentTaskManager.contract, event: "TradeExecuted", logs: logs, sub: sub}, nil
}

// WatchTradeExecuted is a free log subscription operation binding the contract event 0x0dff0d471c9d498b59146ae96d69adade0b56906aab4f3c1a33b450d88b083e9.
//
// Solidity: event TradeExecuted(uint256 indexed taskId, uint256 indexed tradeId, string ticker, uint8 side, uint256 amount, string summary)
func (_AgentTaskManager *AgentTaskManagerFilterer) WatchTradeExecuted(opts *bind.WatchOpts, sink chan<- *AgentTaskManagerTradeExecuted, taskId []*big.Int, tradeId []*big.Int) (event.Subscription, error) {

	var taskIdRule []interface{}
	for _, taskIdItem := range taskId {
		taskIdRule = append(taskIdRule, taskIdItem)
	}
	var tradeIdRule []interface{}
	for _, tradeIdItem := range tradeId {
		tradeIdRule = append(tradeIdRule, tradeIdItem)
	}

	logs, sub, err := _AgentTaskManager.contract.WatchLogs(opts, "TradeExecuted", taskIdRule, tradeIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AgentTaskManagerTradeExecuted)
				if err := _AgentTaskManager.contract.UnpackLog(event, "TradeExecuted", log); err != nil {
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

// ParseTradeExecuted is a log parse operation binding the contract event 0x0dff0d471c9d498b59146ae96d69adade0b56906aab4f3c1a33b450d88b083e9.
//
// Solidity: event TradeExecuted(uint256 indexed taskId, uint256 indexed tradeId, string ticker, uint8 side, uint256 amount, string summary)
func (_AgentTaskManager *AgentTaskManagerFilterer) ParseTradeExecuted(log types.Log) (*AgentTaskManagerTradeExecuted, error) {
	event := new(AgentTaskManagerTradeExecuted)
	if err := _AgentTaskManager.contract.UnpackLog(event, "TradeExecuted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// AgentTaskManagerTradePermissionGrantedIterator is returned from FilterTradePermissionGranted and is used to iterate over the raw logs and unpacked data for TradePermissionGranted events raised by the AgentTaskManager contract.
type AgentTaskManagerTradePermissionGrantedIterator struct {
	Event *AgentTaskManagerTradePermissionGranted // Event containing the contract specifics and raw log

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
func (it *AgentTaskManagerTradePermissionGrantedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(AgentTaskManagerTradePermissionGranted)
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
		it.Event = new(AgentTaskManagerTradePermissionGranted)
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
func (it *AgentTaskManagerTradePermissionGrantedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *AgentTaskManagerTradePermissionGrantedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// AgentTaskManagerTradePermissionGranted represents a TradePermissionGranted event raised by the AgentTaskManager contract.
type AgentTaskManagerTradePermissionGranted struct {
	TaskId      *big.Int
	TotalBudget *big.Int
	ExpiresAt   uint64
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterTradePermissionGranted is a free log retrieval operation binding the contract event 0xc9de1ee51dc7672390c9c7a52e7a69120fa7c94224c9d037519ea1355c01991d.
//
// Solidity: event TradePermissionGranted(uint256 indexed taskId, uint256 totalBudget, uint64 expiresAt)
func (_AgentTaskManager *AgentTaskManagerFilterer) FilterTradePermissionGranted(opts *bind.FilterOpts, taskId []*big.Int) (*AgentTaskManagerTradePermissionGrantedIterator, error) {

	var taskIdRule []interface{}
	for _, taskIdItem := range taskId {
		taskIdRule = append(taskIdRule, taskIdItem)
	}

	logs, sub, err := _AgentTaskManager.contract.FilterLogs(opts, "TradePermissionGranted", taskIdRule)
	if err != nil {
		return nil, err
	}
	return &AgentTaskManagerTradePermissionGrantedIterator{contract: _AgentTaskManager.contract, event: "TradePermissionGranted", logs: logs, sub: sub}, nil
}

// WatchTradePermissionGranted is a free log subscription operation binding the contract event 0xc9de1ee51dc7672390c9c7a52e7a69120fa7c94224c9d037519ea1355c01991d.
//
// Solidity: event TradePermissionGranted(uint256 indexed taskId, uint256 totalBudget, uint64 expiresAt)
func (_AgentTaskManager *AgentTaskManagerFilterer) WatchTradePermissionGranted(opts *bind.WatchOpts, sink chan<- *AgentTaskManagerTradePermissionGranted, taskId []*big.Int) (event.Subscription, error) {

	var taskIdRule []interface{}
	for _, taskIdItem := range taskId {
		taskIdRule = append(taskIdRule, taskIdItem)
	}

	logs, sub, err := _AgentTaskManager.contract.WatchLogs(opts, "TradePermissionGranted", taskIdRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(AgentTaskManagerTradePermissionGranted)
				if err := _AgentTaskManager.contract.UnpackLog(event, "TradePermissionGranted", log); err != nil {
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

// ParseTradePermissionGranted is a log parse operation binding the contract event 0xc9de1ee51dc7672390c9c7a52e7a69120fa7c94224c9d037519ea1355c01991d.
//
// Solidity: event TradePermissionGranted(uint256 indexed taskId, uint256 totalBudget, uint64 expiresAt)
func (_AgentTaskManager *AgentTaskManagerFilterer) ParseTradePermissionGranted(log types.Log) (*AgentTaskManagerTradePermissionGranted, error) {
	event := new(AgentTaskManagerTradePermissionGranted)
	if err := _AgentTaskManager.contract.UnpackLog(event, "TradePermissionGranted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

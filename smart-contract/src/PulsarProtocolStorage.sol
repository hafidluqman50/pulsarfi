// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Initializable} from "@openzeppelin/contracts/proxy/utils/Initializable.sol";
import {UUPSUpgradeable} from "@openzeppelin/contracts/proxy/utils/UUPSUpgradeable.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {PausableUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import {Context} from "@openzeppelin/contracts/utils/Context.sol";
import {ContextUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/ContextUpgradeable.sol";
import {IUniswapV2Router02} from "./interfaces/IUniswapV2Router02.sol";
import {IPoolManager} from "@uniswap/v4-core/src/interfaces/IPoolManager.sol";
import {PoolKey} from "@uniswap/v4-core/src/types/PoolKey.sol";

/// @notice Storage-only base shared by PulsarProtocol and PulsarProtocolOps (the
///         latter called via delegatecall from the proxy, never used standalone).
///         Declaring every state variable ONCE here and having both inherit it
///         guarantees byte-identical slot layout — assigned by the compiler from
///         declaration order, not hand-counted — which is the entire safety
///         property a delegatecall split depends on.
///
///         NEVER reorder, retype, or remove anything below; only ever append new
///         variables at the end, exactly like the rest of this proxy's upgrade
///         history (this file exists purely because PulsarProtocol's own bytecode
///         hit the EIP-170 24,576-byte limit once this session's additions were
///         included — see PulsarProtocolOps for what moved out, and why, and note
///         on gas: every moved function now costs one extra DELEGATECALL per use).
abstract contract PulsarProtocolStorage is Initializable, UUPSUpgradeable, AccessControl, PausableUpgradeable {
    bytes32 public constant CUSTODIAN_ROLE = keccak256("CUSTODIAN_ROLE");
    uint8 public constant THRESHOLD = 3;

    /// @dev V4 pool parameters for a first-mint pool. The 0.3% lpFee accrues to
    ///      the protocol's own full-range position (protocol is the sole LP), so it
    ///      grows protocol-owned liquidity and is separate from the hook fee.
    int24 internal constant MINT_TICK_SPACING = 60;
    uint24 internal constant MINT_LP_FEE = 3000;

    struct MintProposal {
        string ticker;
        string stockName;
        string idxTicker;
        uint256 tokenAmount;
        uint256 idrxAmount;
        bytes32 attestationHash;
        uint8 __deprecatedDestination; // preserve slot — was MintDestination enum, removed in v3
        address requester;
        uint8 approvalCount;
        bool executed;
        address rejectInitiator;
        uint8 rejectCount;
    }

    struct RedeemRequest {
        string ticker;
        address user;
        uint256 tokenAmount;
        uint256 feeIdrx;
        bool processed;
        bool approved;
        address approveInitiator;
        address rejectInitiator;
        uint8 approvalCount;
        uint8 rejectCount;
    }

    IUniswapV2Router02 public router;
    address public idrx;
    address public treasury;
    uint256 public redeemFeeBps;

    mapping(string => address) public stocks;
    string[] internal _tickers;
    mapping(address => bool) public kycApproved;

    mapping(uint256 => MintProposal) public proposals;
    mapping(uint256 => mapping(address => bool)) public hasApproved;
    mapping(string => bool) public hasPendingRequest;
    uint256 public proposalCount;

    mapping(uint256 => RedeemRequest) public redeemRequests;
    uint256 public redeemRequestCount;

    mapping(uint256 => mapping(address => bool)) public hasRejectedMint;
    mapping(uint256 => mapping(address => bool)) public hasApprovedRedeem;
    mapping(uint256 => mapping(address => bool)) public hasRejectedRedeem;

    // Storage slot preserved for UUPS layout compatibility — used only for legacy
    // proposals created before this upgrade. New proposals never write to this mapping.
    mapping(uint256 => uint256) public mintLiquidityFunding;

    // New state below this line — appended, never inserted, to preserve UUPS storage layout.
    uint256 public swapFeeBps;
    uint256 public minimumDistributionThreshold;
    uint256 public accumulatedFees;
    mapping(address => bool) public isActiveCustodian;
    address[] internal _activeCustodians;

    // ─── V4 migration state (appended) ──────────────────────────────────────
    IPoolManager public poolManager;
    address public swapHook;
    mapping(string => PoolKey) public poolKeys; // ticker => V4 pool key
    mapping(string => bool) public isV4Migrated; // ticker => swaps route to V4

    // ─── Size-cutover split (appended, this session) ────────────────────────
    /// @dev Deployed PulsarProtocolOps, invoked via delegatecall for every
    ///      function that moved out of PulsarProtocol's own bytecode.
    address public opsContract;

    event V2ToV4Migrated(string indexed ticker, uint256 idrxRecovered, uint256 stockRecovered);

    // ─── Context resolution ─────────────────────────────────────────────────
    // AccessControl (Context) and PausableUpgradeable (ContextUpgradeable) both
    // define these. Both are stateless and identical, so we resolve explicitly
    // here — once, for every contract that inherits this storage base.

    function _msgSender() internal view virtual override(Context, ContextUpgradeable) returns (address) {
        return msg.sender;
    }

    function _msgData() internal view virtual override(Context, ContextUpgradeable) returns (bytes calldata) {
        return msg.data;
    }

    function _contextSuffixLength() internal view virtual override(Context, ContextUpgradeable) returns (uint256) {
        return 0;
    }
}

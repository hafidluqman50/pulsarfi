// SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

import {Test, console} from "forge-std/Test.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import {PulsarProtocol} from "../src/PulsarProtocol.sol";
import {PulsarProtocolOps} from "../src/PulsarProtocolOps.sol";
import {PulsarStock} from "../src/PulsarStock.sol";
import {PulsarSwapHook} from "../src/v4/PulsarSwapHook.sol";
import {IDRX} from "../src/mocks/IDRX.sol";

import {PoolManager} from "@uniswap/v4-core/src/PoolManager.sol";
import {IPoolManager} from "@uniswap/v4-core/src/interfaces/IPoolManager.sol";
import {Hooks} from "@uniswap/v4-core/src/libraries/Hooks.sol";
import {IHooks} from "@uniswap/v4-core/src/interfaces/IHooks.sol";
import {Currency} from "@uniswap/v4-core/src/types/Currency.sol";
import {FullMath} from "@uniswap/v4-core/src/libraries/FullMath.sol";
import {HookMiner} from "@uniswap/v4-periphery/src/utils/HookMiner.sol";

/// @notice V4-native unit tests. Runs entirely against a locally deployed
///         Uniswap V4 PoolManager plus a HookMiner-mined PulsarSwapHook — no fork
///         needed. The full V2->V4 migration + emergency path is covered separately
///         by test/v4/PulsarProtocolV4Fork.t.sol against the real Arb Sepolia
///         PoolManager.
contract PulsarProtocolTest is Test {
    PulsarProtocol protocol;
    IDRX idrxToken;
    PoolManager poolManager;
    PulsarSwapHook hook;

    address admin  = makeAddr("admin");
    address cust1  = makeAddr("cust1"); // requester throughout tests
    address cust2  = makeAddr("cust2");
    address cust3  = makeAddr("cust3");
    address cust4  = makeAddr("cust4");
    address cust5  = makeAddr("cust5");
    address trader = makeAddr("trader");

    // 1 000 pTokens (18 decimals)
    uint256 constant TOKEN_AMOUNT = 1_000 * 1e18;
    // 25 000.00 IDRX (2 decimals → 2_500_000 units)
    uint256 constant IDRX_AMOUNT  = 2_500_000;
    // Give cust1 enough IDRX for multiple operations
    uint256 constant CUST1_IDRX_BALANCE = 100_000_000; // 1 000 000.00 IDRX

    bytes32 constant ATTEST = keccak256("test-attest-1");
    uint256 idrxId; // ERC-6909 claim id for IDRX in the PoolManager

    // ─── Setup ────────────────────────────────────────────────────────────────

    function setUp() public {
        // IDRX mock
        vm.prank(admin);
        idrxToken = new IDRX(admin);
        idrxId = uint256(uint160(address(idrxToken)));

        // Local Uniswap V4 PoolManager
        poolManager = new PoolManager(admin);

        // PulsarProtocol behind a UUPS proxy. executeMint/swap are V4-native and
        // never touch the V2 router post-cutover, so a placeholder router address is
        // fine here (migrateV2ToV4 is exercised by the fork test).
        address[] memory custodians = new address[](5);
        custodians[0] = cust1;
        custodians[1] = cust2;
        custodians[2] = cust3;
        custodians[3] = cust4;
        custodians[4] = cust5;

        PulsarProtocol impl = new PulsarProtocol();
        ERC1967Proxy proxy = new ERC1967Proxy(
            address(impl),
            abi.encodeCall(
                PulsarProtocol.initialize,
                (admin, makeAddr("router"), address(idrxToken), custodians, admin)
            )
        );
        protocol = PulsarProtocol(address(proxy));

        // Mine + deploy the fee hook. feeConfig = protocol (reads swapFeeBps live);
        // pauser = protocol (so emergencyWithdrawV4 can pause it on-chain).
        uint160 flags = uint160(
            Hooks.BEFORE_SWAP_FLAG | Hooks.AFTER_SWAP_FLAG | Hooks.BEFORE_SWAP_RETURNS_DELTA_FLAG
                | Hooks.AFTER_SWAP_RETURNS_DELTA_FLAG
        );
        bytes memory args =
            abi.encode(IPoolManager(address(poolManager)), address(protocol), address(idrxToken), admin, address(protocol));
        (address hookAddr, bytes32 salt) =
            HookMiner.find(address(this), flags, type(PulsarSwapHook).creationCode, args);
        hook = new PulsarSwapHook{salt: salt}(
            IPoolManager(address(poolManager)), address(protocol), address(idrxToken), admin, address(protocol)
        );
        assertEq(address(hook), hookAddr, "hook must deploy at the mined address");

        vm.startPrank(admin);
        protocol.configureV4(address(poolManager), address(hook));
        // Route swept hook fees to the protocol so collectV4Fees can redeem them.
        hook.setFeeRecipient(address(protocol));
        // Wire the ops delegatecall target (see PulsarProtocolStorage's docs —
        // several functions moved out to keep PulsarProtocol under EIP-170).
        protocol.setOpsContract(address(new PulsarProtocolOps()));
        vm.stopPrank();

        // Fund cust1 and trader with IDRX
        vm.startPrank(admin);
        idrxToken.mint(cust1,  CUST1_IDRX_BALANCE);
        idrxToken.mint(trader, 50_000_000); // 500 000.00 IDRX for swap tests
        vm.stopPrank();

        // KYC approve trader (approveKYC requires CUSTODIAN_ROLE)
        vm.prank(cust1);
        protocol.approveKYC(trader);
    }

    // ─── Helpers ──────────────────────────────────────────────────────────────

    function _sqrt(uint256 x) internal pure returns (uint256 y) {
        if (x == 0) return 0;
        uint256 z = (x + 1) / 2;
        y = x;
        while (z < y) {
            y = z;
            z = (x / z + z) / 2;
        }
    }

    /// Canonical initial price passed to executeMint: sqrt(IDRX_raw / stock_raw) * 2^96.
    /// Orientation-independent — the contract flips it to the pool's currency order.
    function _canonicalSqrtPriceX96(uint256 idrxAmt, uint256 stockAmt) internal pure returns (uint160) {
        uint256 ratioX192 = FullMath.mulDiv(idrxAmt, uint256(1) << 192, stockAmt);
        return uint160(_sqrt(ratioX192));
    }

    function _defaultSqrtPrice() internal pure returns (uint160) {
        return _canonicalSqrtPriceX96(IDRX_AMOUNT, TOKEN_AMOUNT);
    }

    /// Submit a requestMint from cust1 and return proposalId.
    function _requestMint() internal returns (uint256 proposalId) {
        vm.prank(cust1);
        proposalId = protocol.requestMint("BUMIP", "Pulsar Bumi Resources", "BUMI", TOKEN_AMOUNT, IDRX_AMOUNT, ATTEST);
    }

    /// Reach threshold (cust2 + cust3 approve) so cust1 can executeMint.
    function _reachThreshold(uint256 proposalId) internal {
        vm.prank(cust2);
        protocol.approveMint(proposalId);
        vm.prank(cust3);
        protocol.approveMint(proposalId);
    }

    /// Full first mint: request → approve × 2 → executeMint (creates the V4 pool).
    function _mintAndPool() internal returns (address stockAddress) {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId, _defaultSqrtPrice());
        vm.stopPrank();
        stockAddress = protocol.stocks("BUMIP");
    }

    /// Retail-style buy through the V4 pool; returns stock received.
    function _buyStock(address who, uint256 amountIn) internal returns (uint256 received) {
        address stockAddress = protocol.stocks("BUMIP");
        uint256 before = PulsarStock(stockAddress).balanceOf(who);
        vm.startPrank(who);
        idrxToken.approve(address(protocol), amountIn);
        protocol.swapV4("BUMIP", amountIn, 0, true);
        vm.stopPrank();
        received = PulsarStock(stockAddress).balanceOf(who) - before;
    }

    // ─── requestMint ──────────────────────────────────────────────────────────

    function test_requestMint_doesNotPullIDRX() public {
        uint256 balanceBefore = idrxToken.balanceOf(cust1);
        _requestMint();
        assertEq(idrxToken.balanceOf(cust1), balanceBefore, "requestMint must not pull IDRX from requester");
    }

    function test_requestMint_setsProposalFields() public {
        uint256 proposalId = _requestMint();

        (
            string memory ticker,
            ,
            ,
            uint256 tokenAmount,
            uint256 idrxAmount,
            ,
            ,
            address requester,
            uint8 approvalCount,
            bool executed,
            ,
        ) = protocol.proposals(proposalId);

        assertEq(ticker,        "BUMIP");
        assertEq(tokenAmount,   TOKEN_AMOUNT);
        assertEq(idrxAmount,    IDRX_AMOUNT);
        assertEq(requester,     cust1);
        assertEq(approvalCount, 1);
        assertFalse(executed);
    }

    function test_requestMint_preventsDoublePending() public {
        _requestMint();
        vm.expectRevert(abi.encodeWithSelector(MintRequestPending.selector, "BUMIP"));
        vm.prank(cust1);
        protocol.requestMint("BUMIP", "Pulsar Bumi Resources", "BUMI", TOKEN_AMOUNT, IDRX_AMOUNT, ATTEST);
    }

    // ─── approveMint ──────────────────────────────────────────────────────────

    function test_approveMint_incrementsCount() public {
        uint256 proposalId = _requestMint();
        vm.prank(cust2);
        protocol.approveMint(proposalId);
        (,,,,,,,, uint8 approvalCount,,,) = protocol.proposals(proposalId);
        assertEq(approvalCount, 2);
    }

    function test_approveMint_rejectsDouble() public {
        uint256 proposalId = _requestMint();
        vm.prank(cust2);
        protocol.approveMint(proposalId);
        vm.expectRevert(abi.encodeWithSelector(AlreadyApproved.selector, proposalId, cust2));
        vm.prank(cust2);
        protocol.approveMint(proposalId);
    }

    // ─── executeMint (V4-native) ───────────────────────────────────────────────

    function test_executeMint_pullsIDRXAtExecution() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);

        uint256 balanceBeforeExec = idrxToken.balanceOf(cust1);

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId, _defaultSqrtPrice());
        vm.stopPrank();

        uint256 balanceAfterExec = idrxToken.balanceOf(cust1);
        assertLe(balanceBeforeExec - balanceAfterExec, IDRX_AMOUNT, "must pull at most IDRX_AMOUNT");
        assertLt(balanceAfterExec, balanceBeforeExec, "must pull some IDRX from requester");
    }

    function test_executeMint_createsV4Pool() public {
        _mintAndPool();

        // A V4 pool key is now registered and the ticker is live on V4.
        (,,, , address hooks) = _poolKey("BUMIP");
        assertEq(hooks, address(hook), "V4 pool must carry the fee hook after executeMint");
        assertTrue(protocol.isV4Migrated("BUMIP"), "ticker must be V4-native after first mint");
    }

    function test_executeMint_emitsV4PoolCreated() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        vm.expectEmit(true, false, false, false, address(protocol));
        emit PulsarProtocol.V4PoolCreated("BUMIP", 0);
        protocol.executeMint(proposalId, _defaultSqrtPrice());
        vm.stopPrank();
    }

    function test_executeMint_secondMintEmitsV4LiquidityAdded() public {
        _mintAndPool();

        // Second mint adds to the existing V4 pool (no new pool created).
        vm.prank(cust2);
        uint256 secondId = protocol.requestMint("BUMIP", "Pulsar Bumi Resources", "BUMI", TOKEN_AMOUNT, IDRX_AMOUNT, ATTEST);
        vm.prank(cust1);
        protocol.approveMint(secondId);
        vm.prank(cust3);
        protocol.approveMint(secondId);

        vm.prank(admin);
        idrxToken.mint(cust2, IDRX_AMOUNT);

        vm.startPrank(cust2);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        vm.expectEmit(true, false, false, false, address(protocol));
        emit PulsarProtocol.V4LiquidityAdded("BUMIP", 0, 0);
        protocol.executeMint(secondId, _defaultSqrtPrice());
        vm.stopPrank();
    }

    function test_executeMint_revertsWithoutIDRXApproval() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);
        vm.prank(cust1);
        vm.expectRevert();
        protocol.executeMint(proposalId, _defaultSqrtPrice());
    }

    function test_executeMint_revertsIfNotRequester() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);
        vm.expectRevert(abi.encodeWithSelector(NotRequester.selector, proposalId, cust2));
        vm.prank(cust2);
        protocol.executeMint(proposalId, _defaultSqrtPrice());
    }

    function test_executeMint_revertsIfThresholdNotMet() public {
        uint256 proposalId = _requestMint();
        vm.prank(cust1);
        vm.expectRevert(abi.encodeWithSelector(ThresholdNotMet.selector, proposalId, 1, 3));
        protocol.executeMint(proposalId, _defaultSqrtPrice());
    }

    function test_executeMint_revertsIfAlreadyExecuted() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId, _defaultSqrtPrice());
        vm.stopPrank();

        vm.expectRevert(abi.encodeWithSelector(ProposalAlreadyExecuted.selector, proposalId));
        vm.prank(cust1);
        protocol.executeMint(proposalId, _defaultSqrtPrice());
    }

    // ─── rejectMint / executeRejectMint ──────────────────────────────────────

    function test_executeRejectMint_noFunding_noRefund() public {
        uint256 proposalId = _requestMint();
        uint256 balanceBefore = idrxToken.balanceOf(cust1);

        vm.prank(cust1);
        protocol.rejectMint(proposalId);
        vm.prank(cust2);
        protocol.rejectMint(proposalId);
        vm.prank(cust3);
        protocol.rejectMint(proposalId);

        vm.prank(cust1);
        protocol.executeRejectMint(proposalId);

        assertEq(idrxToken.balanceOf(cust1), balanceBefore, "no IDRX was locked so balance must be unchanged");
        assertEq(protocol.mintLiquidityFunding(proposalId), 0);
    }

    function test_executeRejectMint_legacyFunding_refundsRequester() public {
        uint256 proposalId = _requestMint();

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.fundMintLiquidity(proposalId, IDRX_AMOUNT);
        vm.stopPrank();

        assertEq(protocol.mintLiquidityFunding(proposalId), IDRX_AMOUNT);
        uint256 balanceBeforeReject = idrxToken.balanceOf(cust1);

        vm.prank(cust1);
        protocol.rejectMint(proposalId);
        vm.prank(cust2);
        protocol.rejectMint(proposalId);
        vm.prank(cust3);
        protocol.rejectMint(proposalId);

        vm.prank(cust1);
        protocol.executeRejectMint(proposalId);

        assertEq(idrxToken.balanceOf(cust1), balanceBeforeReject + IDRX_AMOUNT, "locked IDRX must be refunded");
        assertEq(protocol.mintLiquidityFunding(proposalId), 0, "funding mapping must be cleared");
    }

    function test_executeRejectMint_clearsPendingRequest() public {
        uint256 proposalId = _requestMint();
        assertTrue(protocol.hasPendingRequest("BUMIP"));

        vm.prank(cust1);
        protocol.rejectMint(proposalId);
        vm.prank(cust2);
        protocol.rejectMint(proposalId);
        vm.prank(cust3);
        protocol.rejectMint(proposalId);

        vm.prank(cust1);
        protocol.executeRejectMint(proposalId);
        assertFalse(protocol.hasPendingRequest("BUMIP"), "pending flag must be cleared after rejection");
    }

    function test_executeRejectMint_revertsIfThresholdNotMet() public {
        uint256 proposalId = _requestMint();
        vm.prank(cust1);
        protocol.rejectMint(proposalId);
        vm.prank(cust2);
        protocol.rejectMint(proposalId);

        vm.expectRevert(abi.encodeWithSelector(ThresholdNotMet.selector, proposalId, 2, 3));
        vm.prank(cust1);
        protocol.executeRejectMint(proposalId);
    }

    // ─── Legacy: executeMint with pre-funded IDRX ────────────────────────────

    function test_executeMint_legacyFunding_usesExistingIDRX() public {
        uint256 proposalId = _requestMint();

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.fundMintLiquidity(proposalId, IDRX_AMOUNT);
        vm.stopPrank();

        _reachThreshold(proposalId);
        uint256 cust1BalanceBefore = idrxToken.balanceOf(cust1);

        // No additional approve — IDRX already funded.
        vm.prank(cust1);
        protocol.executeMint(proposalId, _defaultSqrtPrice());

        assertEq(idrxToken.balanceOf(cust1), cust1BalanceBefore, "fully-funded proposal must not pull additional IDRX");
        assertTrue(protocol.isV4Migrated("BUMIP"), "V4 pool must be seeded");
        assertEq(protocol.mintLiquidityFunding(proposalId), 0);
    }

    function test_executeMint_legacyPartialFunding_pullsShortfall() public {
        uint256 proposalId = _requestMint();
        uint256 partialFund = IDRX_AMOUNT / 2;

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), partialFund);
        protocol.fundMintLiquidity(proposalId, partialFund);
        vm.stopPrank();

        _reachThreshold(proposalId);
        uint256 balanceBefore = idrxToken.balanceOf(cust1);

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId, _defaultSqrtPrice());
        vm.stopPrank();

        uint256 pulled = balanceBefore - idrxToken.balanceOf(cust1);
        assertLe(pulled, IDRX_AMOUNT - partialFund, "must not pull more than the shortfall");
        assertTrue(protocol.isV4Migrated("BUMIP"), "V4 pool must be seeded");
    }

    // ─── Swap (V4) ──────────────────────────────────────────────────────────

    function test_swapV4_buyStock_afterPoolCreated() public {
        _mintAndPool();
        uint256 received = _buyStock(trader, 250_000);
        assertGt(received, 0, "trader must receive pStock tokens");
    }

    function test_swapV4_sellStock_afterPoolCreated() public {
        address stockAddress = _mintAndPool();
        _buyStock(trader, 250_000);
        uint256 stockBalance = PulsarStock(stockAddress).balanceOf(trader);
        assertGt(stockBalance, 0);

        uint256 idrxBefore = idrxToken.balanceOf(trader);
        vm.startPrank(trader);
        PulsarStock(stockAddress).approve(address(protocol), stockBalance);
        protocol.swapV4("BUMIP", stockBalance, 0, false);
        vm.stopPrank();

        assertGt(idrxToken.balanceOf(trader), idrxBefore, "trader must receive IDRX after selling");
    }

    function test_swapV4_revertsForUnknownTicker() public {
        vm.prank(trader);
        vm.expectRevert(abi.encodeWithSelector(V4PoolNotFound.selector, "XXXX"));
        protocol.swapV4("XXXX", 1000, 0, true);
    }

    // ─── KYC / redeem ────────────────────────────────────────────────────────

    function test_requestRedeem_revertsWithoutKYC() public {
        address stockAddress = _mintAndPool();

        address noKycUser = makeAddr("noKyc");
        vm.prank(admin);
        idrxToken.mint(noKycUser, 100_000);
        vm.prank(noKycUser);
        PulsarStock(stockAddress).approve(address(protocol), 1e18);

        vm.prank(noKycUser);
        vm.expectRevert(abi.encodeWithSelector(KYCRequired.selector, noKycUser));
        protocol.requestRedeem("BUMIP", 1e18);
    }

    /// requestRedeem is permissionless: any KYC-approved wallet can call it directly.
    function test_requestRedeem_permissionlessForNonCustodianKycUser() public {
        address stockAddress = _mintAndPool();

        address kycUser = makeAddr("kycUser");
        vm.prank(admin);
        idrxToken.mint(kycUser, 100_000);
        vm.prank(cust1);
        protocol.approveKYC(kycUser);

        uint256 stockBalance = _buyStock(kycUser, 100_000);
        vm.startPrank(kycUser);
        PulsarStock(stockAddress).approve(address(protocol), stockBalance);
        protocol.requestRedeem("BUMIP", stockBalance);
        vm.stopPrank();

        assertEq(protocol.redeemRequestCount(), 1, "non-custodian KYC user must be able to requestRedeem directly");
    }

    /// The V4 spot-price quote must value stock in raw IDRX correctly across the
    /// 2-decimal / 18-decimal gap. At the seed price 1000 stock == 25 000.00 IDRX,
    /// so 1 whole stock (1e18) ≈ 2 500 raw IDRX (25.00 IDRX).
    function test_requestRedeem_v4QuoteChargesDecimalAwareFee() public {
        address stockAddress = _mintAndPool();

        vm.prank(admin);
        protocol.setRedeemFeeBps(100); // 1%

        address kycUser = makeAddr("kycUser2");
        vm.prank(cust1);
        protocol.approveKYC(kycUser);
        vm.prank(admin);
        idrxToken.mint(kycUser, 1_000_000);

        // Give the user exactly 1 whole stock via a direct transfer from protocol
        // holdings would bypass the pool; instead buy a known amount, then redeem it.
        uint256 stockBought = _buyStock(kycUser, 300_000);
        assertGt(stockBought, 0);

        uint256 userIdrxBefore = idrxToken.balanceOf(kycUser);
        vm.startPrank(kycUser);
        PulsarStock(stockAddress).approve(address(protocol), stockBought);
        idrxToken.approve(address(protocol), type(uint256).max);
        protocol.requestRedeem("BUMIP", stockBought);
        vm.stopPrank();

        uint256 requestId = protocol.redeemRequestCount() - 1;
        (,, , uint256 lockedFee,,,,,,) = protocol.redeemRequests(requestId);

        // Fee must be ~1% of the stock's IDRX spot value and locked from the user.
        assertGt(lockedFee, 0, "redeem fee must be quoted from the V4 spot price");
        assertEq(userIdrxBefore - idrxToken.balanceOf(kycUser), lockedFee, "exactly the quoted fee must be locked");
    }

    // ─── Access control ───────────────────────────────────────────────────────

    function test_nonCustodian_cannotRequestMint() public {
        vm.prank(trader);
        vm.expectRevert();
        protocol.requestMint("BUMIP", "Pulsar Bumi Resources", "BUMI", TOKEN_AMOUNT, IDRX_AMOUNT, ATTEST);
    }

    function test_nonCustodian_cannotApproveKYC() public {
        vm.prank(trader);
        vm.expectRevert();
        protocol.approveKYC(trader);
    }

    // ─── Circuit breaker ────────────────────────────────────────────────────

    function test_pause_blocksExecuteMint() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);

        vm.prank(cust1);
        protocol.pause();

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        vm.expectRevert(); // EnforcedPause
        protocol.executeMint(proposalId, _defaultSqrtPrice());
        vm.stopPrank();
    }

    function test_pause_thenUnpause_restoresExecuteMint() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);

        vm.prank(cust1);
        protocol.pause();
        vm.prank(admin);
        protocol.unpause();

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId, _defaultSqrtPrice()); // must succeed after unpause
        vm.stopPrank();

        assertFalse(protocol.paused());
    }

    function test_pause_onlyCustodian() public {
        vm.prank(trader);
        vm.expectRevert();
        protocol.pause();
    }

    function test_unpause_onlyAdmin() public {
        vm.prank(cust1);
        protocol.pause();
        vm.prank(cust1);
        vm.expectRevert();
        protocol.unpause();
    }

    function test_pause_leavesRejectPathOpen() public {
        uint256 proposalId = _requestMint();

        vm.prank(cust1);
        protocol.pause();

        vm.prank(cust1);
        protocol.rejectMint(proposalId);
        vm.prank(cust2);
        protocol.rejectMint(proposalId);
        vm.prank(cust3);
        protocol.rejectMint(proposalId);

        vm.prank(cust1);
        protocol.executeRejectMint(proposalId); // must work despite pause
        assertFalse(protocol.hasPendingRequest("BUMIP"));
    }

    function test_pause_blocksSwapV4() public {
        _mintAndPool();
        vm.prank(cust1);
        protocol.pause();

        vm.startPrank(trader);
        idrxToken.approve(address(protocol), 100_000);
        vm.expectRevert(); // EnforcedPause
        protocol.swapV4("BUMIP", 100_000, 0, true);
        vm.stopPrank();
    }

    // ─── Pool-level circuit breaker (pauseHook / unpauseHook) ─────────────────

    /// pauseHook must halt swaps at the POOL level even though the protocol
    /// itself is not paused — narrower than emergencyWithdrawV4 (no liquidity
    /// pulled) and distinct from protocol.pause() (which only blocks the
    /// protocol's own entry points).
    function test_pauseHook_haltsSwapV4WithoutPullingLiquidity() public {
        _mintAndPool();

        vm.prank(cust1);
        protocol.pauseHook();

        assertTrue(hook.paused(), "hook must be paused");
        assertFalse(protocol.paused(), "protocol itself must remain unpaused");
        assertTrue(protocol.isV4Migrated("BUMIP"), "liquidity must NOT be pulled by pauseHook");

        vm.startPrank(trader);
        idrxToken.approve(address(protocol), 100_000);
        vm.expectRevert();
        protocol.swapV4("BUMIP", 100_000, 0, true);
        vm.stopPrank();
    }

    function test_pauseHook_onlyCustodian() public {
        _mintAndPool();
        vm.prank(trader);
        vm.expectRevert();
        protocol.pauseHook();
    }

    function test_unpauseHook_onlyAdmin() public {
        _mintAndPool();
        vm.prank(cust1);
        protocol.pauseHook();

        vm.prank(cust1); // a custodian (non-admin) cannot unpause the hook
        vm.expectRevert();
        protocol.unpauseHook();
    }

    function test_unpauseHook_restoresSwapV4() public {
        _mintAndPool();

        vm.prank(cust1);
        protocol.pauseHook();
        vm.prank(admin);
        protocol.unpauseHook();

        assertFalse(hook.paused(), "hook must be unpaused");

        uint256 received = _buyStock(trader, 100_000);
        assertGt(received, 0, "swap must succeed again after unpauseHook");
    }

    // ─── Swap fee (hook + collectV4Fees) ──────────────────────────────────────

    /// Buy side: the hook skims the protocol fee off the IDRX input as an ERC-6909
    /// claim; collectV4Fees redeems it into accumulatedFees, exactly bps of input.
    function test_swapV4_buyStock_feeAccruesToHookThenProtocol() public {
        _mintAndPool();

        vm.prank(admin);
        protocol.setSwapFeeBps(20); // 0.2%

        uint256 swapIn = 250_000;
        uint256 expectedFee = (swapIn * 20) / 10_000;

        _buyStock(trader, swapIn);

        // Fee sits on the hook as an IDRX 6909 claim; not yet in accumulatedFees.
        assertEq(poolManager.balanceOf(address(hook), idrxId), expectedFee, "hook must hold exactly the buy-side IDRX fee");
        assertEq(protocol.accumulatedFees(), 0, "fee is not booked until collectV4Fees");

        protocol.collectV4Fees();
        assertEq(protocol.accumulatedFees(), expectedFee, "collectV4Fees must book the exact buy-side fee");
        assertEq(poolManager.balanceOf(address(hook), idrxId), 0, "hook claim must be swept");
    }

    /// Sell side: the hook takes the fee from the IDRX output via afterSwap.
    function test_swapV4_sellStock_feeAccruesFromOutput() public {
        address stockAddress = _mintAndPool();

        // Buy first (with fee already on) so trader holds stock to sell.
        vm.prank(admin);
        protocol.setSwapFeeBps(20); // 0.2%
        _buyStock(trader, 250_000);
        protocol.collectV4Fees(); // clear buy-side fee so we isolate the sell fee
        uint256 feesBefore = protocol.accumulatedFees();

        uint256 stockBalance = PulsarStock(stockAddress).balanceOf(trader);
        uint256 idrxBefore = idrxToken.balanceOf(trader);

        vm.startPrank(trader);
        PulsarStock(stockAddress).approve(address(protocol), stockBalance);
        protocol.swapV4("BUMIP", stockBalance, 0, false);
        vm.stopPrank();

        uint256 received = idrxToken.balanceOf(trader) - idrxBefore;
        assertGt(received, 0, "trader must still receive net IDRX after fee");

        protocol.collectV4Fees();
        uint256 sellFee = protocol.accumulatedFees() - feesBefore;
        assertGt(sellFee, 0, "sell-side protocol fee must be recorded");
        // sell fee == floor((received + sellFee) * bps / 10000)
        assertEq(sellFee, ((received + sellFee) * 20) / 10_000, "sell-side fee must match bps of gross output");
    }

    function test_swapV4_zeroFeeBps_noFeeAccrues() public {
        address stockAddress = _mintAndPool();

        // swapFeeBps stays at default 0.
        _buyStock(trader, 250_000);
        uint256 stockBalance = PulsarStock(stockAddress).balanceOf(trader);

        uint256 idrxBefore = idrxToken.balanceOf(trader);
        vm.startPrank(trader);
        PulsarStock(stockAddress).approve(address(protocol), stockBalance);
        protocol.swapV4("BUMIP", stockBalance, 0, false);
        vm.stopPrank();

        assertGt(idrxToken.balanceOf(trader), idrxBefore, "trader must receive IDRX with no protocol fee");
        assertEq(poolManager.balanceOf(address(hook), idrxId), 0, "no hook fee claim when bps is 0");
        protocol.collectV4Fees();
        assertEq(protocol.accumulatedFees(), 0, "no fee should accrue when swapFeeBps is 0");
    }

    // ─── Fuzz: swap fee ───────────────────────────────────────────────────────

    function testFuzz_swapV4_buyStock_feeMatchesFormula(uint256 feeBps, uint256 swapIn) public {
        feeBps = bound(feeBps, 0, 1000); // max 10%, contract-enforced cap
        swapIn = bound(swapIn, 10_000, 500_000); // within pool depth so no price-limit clipping

        _mintAndPool();
        vm.prank(admin);
        protocol.setSwapFeeBps(feeBps);

        uint256 expectedFee = (swapIn * feeBps) / 10_000;
        _buyStock(trader, swapIn);

        protocol.collectV4Fees();
        assertEq(protocol.accumulatedFees(), expectedFee, "buy-side fee must match bps formula for any feeBps/amount");
        assertLe(expectedFee, swapIn, "fee must never exceed the swapped-in amount");
    }

    function testFuzz_swapV4_sellStock_feeMatchesFormula(uint256 feeBps) public {
        feeBps = bound(feeBps, 0, 1000);

        address stockAddress = _mintAndPool();
        _buyStock(trader, 250_000);
        uint256 stockBalance = PulsarStock(stockAddress).balanceOf(trader);

        vm.prank(admin);
        protocol.setSwapFeeBps(feeBps);

        uint256 idrxBefore = idrxToken.balanceOf(trader);
        vm.startPrank(trader);
        PulsarStock(stockAddress).approve(address(protocol), stockBalance);
        protocol.swapV4("BUMIP", stockBalance, 0, false);
        vm.stopPrank();

        uint256 received = idrxToken.balanceOf(trader) - idrxBefore;
        protocol.collectV4Fees();
        uint256 feeCollected = protocol.accumulatedFees();
        uint256 rawOut = received + feeCollected;

        assertEq(feeCollected, (rawOut * feeBps) / 10_000, "sell-side fee must match bps formula for any feeBps");
        assertLe(feeCollected, rawOut, "fee must never exceed the raw swap output");
    }

    // ─── Fuzz: distributeFees conservation ─────────────────────────────────────

    function testFuzz_distributeFees_conservesTotalAcrossFeeRates(uint256 feeBps) public {
        feeBps = bound(feeBps, 1, 1000);

        _mintAndPool();
        vm.prank(admin);
        protocol.setSwapFeeBps(feeBps);

        _buyStock(trader, 250_000);
        protocol.collectV4Fees();

        uint256 fees = protocol.accumulatedFees();
        vm.assume(fees > 0);

        uint256 treasuryBefore = idrxToken.balanceOf(admin);
        uint256 cust1Before = idrxToken.balanceOf(cust1);
        uint256 cust2Before = idrxToken.balanceOf(cust2);
        uint256 cust3Before = idrxToken.balanceOf(cust3);

        protocol.distributeFees();

        uint256 treasuryGain = idrxToken.balanceOf(admin) - treasuryBefore;
        uint256 cust1Gain = idrxToken.balanceOf(cust1) - cust1Before;
        uint256 cust2Gain = idrxToken.balanceOf(cust2) - cust2Before;
        uint256 cust3Gain = idrxToken.balanceOf(cust3) - cust3Before;

        assertEq(protocol.accumulatedFees(), 0, "must reset after distribution");
        assertEq(
            treasuryGain + cust1Gain + cust2Gain + cust3Gain,
            fees,
            "distribution must conserve the full fee total exactly, for any feeBps"
        );
        assertEq(cust1Gain, cust2Gain, "equal split among active custodians");
        assertEq(cust2Gain, cust3Gain, "equal split among active custodians");
    }

    // ─── distributeFees ───────────────────────────────────────────────────────

    function test_distributeFees_revertsBelowThreshold() public {
        vm.prank(admin);
        protocol.setMinimumDistributionThreshold(1_000_000);

        vm.expectRevert(abi.encodeWithSelector(BelowDistributionThreshold.selector, 0, 1_000_000));
        protocol.distributeFees();
    }

    function test_distributeFees_splitsToTreasuryAndActiveCustodians() public {
        _mintAndPool();

        vm.prank(admin);
        protocol.setSwapFeeBps(20); // 0.2%

        _buyStock(trader, 250_000);
        protocol.collectV4Fees();

        uint256 fees = protocol.accumulatedFees();
        assertGt(fees, 0);

        // Active custodians: cust1 (requester) + cust2, cust3 (approvers) = 3.
        uint256 expectedTreasuryShare = (fees * 30) / 100;
        uint256 expectedCustodianPool = fees - expectedTreasuryShare;
        uint256 expectedPerCustodian = expectedCustodianPool / 3;
        uint256 expectedRemainder = expectedCustodianPool - (expectedPerCustodian * 3);

        uint256 treasuryBefore = idrxToken.balanceOf(admin); // treasury == admin
        uint256 cust1Before = idrxToken.balanceOf(cust1);

        protocol.distributeFees();

        assertEq(protocol.accumulatedFees(), 0, "accumulatedFees must reset after distribution");
        assertEq(idrxToken.balanceOf(admin) - treasuryBefore, expectedTreasuryShare + expectedRemainder, "treasury gets 30% + remainder");
        assertEq(idrxToken.balanceOf(cust1) - cust1Before, expectedPerCustodian, "active custodian gets an equal 1/3 share");
    }

    /// A redeem fee still locked as pending escrow must never be swept by
    /// distributeFees, since it may still need to be refunded on reject.
    function test_distributeFees_doesNotTouchPendingRedeemEscrow() public {
        address stockAddress = _mintAndPool();

        vm.prank(admin);
        protocol.setRedeemFeeBps(100); // 1% exit fee

        _buyStock(trader, 250_000);
        uint256 stockBalance = PulsarStock(stockAddress).balanceOf(trader);

        // trader requests redeem — locks stock + redeem fee IDRX as PENDING escrow.
        vm.startPrank(trader);
        PulsarStock(stockAddress).approve(address(protocol), stockBalance);
        idrxToken.approve(address(protocol), type(uint256).max);
        protocol.requestRedeem("BUMIP", stockBalance);
        vm.stopPrank();

        uint256 requestId = protocol.redeemRequestCount() - 1;
        (,, , uint256 lockedFee,,,,,,) = protocol.redeemRequests(requestId);
        assertGt(lockedFee, 0, "redeem fee must be locked pending a decision");
        assertEq(protocol.accumulatedFees(), 0, "pending redeem fee must NOT be in accumulatedFees yet");

        // Separately generate confirmed swap-fee revenue.
        vm.prank(admin);
        protocol.setSwapFeeBps(20); // 0.2%
        _buyStock(trader, 250_000);
        protocol.collectV4Fees();

        uint256 confirmedFees = protocol.accumulatedFees();
        assertGt(confirmedFees, 0, "swap fee must accrue separately from the pending redeem escrow");

        protocol.distributeFees();
        assertEq(protocol.accumulatedFees(), 0, "confirmed fees must be distributed");

        // Reject the pending redeem — must still refund the FULL locked fee.
        vm.prank(cust1);
        protocol.rejectRedeem(requestId);
        vm.prank(cust2);
        protocol.rejectRedeem(requestId);
        vm.prank(cust3);
        protocol.rejectRedeem(requestId);

        uint256 traderIdrxBefore = idrxToken.balanceOf(trader);
        uint256 traderStockBefore = PulsarStock(stockAddress).balanceOf(trader);

        vm.prank(cust1); // first rejecter
        protocol.executeReject(requestId);

        assertEq(
            idrxToken.balanceOf(trader),
            traderIdrxBefore + lockedFee,
            "locked redeem fee must be fully refunded even after an unrelated distributeFees call"
        );
        assertEq(
            PulsarStock(stockAddress).balanceOf(trader),
            traderStockBefore + stockBalance,
            "locked stock must be fully refunded"
        );
    }

    function test_distributeFees_revertsWithNoActiveCustodians() public {
        address[] memory noCustodians = new address[](0);
        PulsarProtocol freshImpl = new PulsarProtocol();
        ERC1967Proxy freshProxy = new ERC1967Proxy(
            address(freshImpl),
            abi.encodeCall(PulsarProtocol.initialize, (admin, makeAddr("router"), address(idrxToken), noCustodians, admin))
        );
        PulsarProtocol fresh = PulsarProtocol(address(freshProxy));
        PulsarProtocolOps freshOps = new PulsarProtocolOps();
        vm.prank(admin);
        fresh.setOpsContract(address(freshOps));

        vm.expectRevert(NoActiveCustodians.selector);
        fresh.distributeFees();
    }

    // ─── collectV4Fees edge cases ──────────────────────────────────────────────

    function test_collectV4Fees_noFees_isNoOp() public {
        _mintAndPool();
        protocol.collectV4Fees(); // no swaps yet
        assertEq(protocol.accumulatedFees(), 0, "collecting with no accrued fees must be a no-op");
    }

    // ─── Internal helpers ──────────────────────────────────────────────────────

    /// Reads the stored V4 PoolKey fields for a ticker via the public mapping getter.
    function _poolKey(string memory ticker)
        internal
        view
        returns (address currency0, address currency1, uint24 fee, int24 tickSpacing, address hooks)
    {
        (Currency c0, Currency c1, uint24 f, int24 ts, IHooks h) = protocol.poolKeys(ticker);
        return (Currency.unwrap(c0), Currency.unwrap(c1), f, ts, address(h));
    }
}

// Bring custom error selectors into scope for vm.expectRevert
error MintRequestPending(string ticker);
error AlreadyApproved(uint256 proposalId, address custodian);
error NotRequester(uint256 proposalId, address caller);
error ThresholdNotMet(uint256 proposalId, uint8 current, uint8 required);
error ProposalAlreadyExecuted(uint256 proposalId);
error KYCRequired(address wallet);
error BelowDistributionThreshold(uint256 balance, uint256 threshold);
error NoActiveCustodians();
error V4PoolNotFound(string ticker);

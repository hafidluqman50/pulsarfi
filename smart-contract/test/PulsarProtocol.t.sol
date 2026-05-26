// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Test, console} from "forge-std/Test.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import {PulsarProtocol} from "../src/PulsarProtocol.sol";
import {PulsarStock} from "../src/PulsarStock.sol";
import {IDRX} from "../src/mocks/IDRX.sol";
import {IUniswapV2Factory} from "../src/interfaces/IUniswapV2Factory.sol";

contract PulsarProtocolTest is Test {
    PulsarProtocol protocol;
    IDRX idrxToken;

    address uniswapFactory;
    address uniswapRouter;

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

    // ─── Setup ────────────────────────────────────────────────────────────────

    // Deploy Uniswap V2 from official pre-compiled artifacts so the init code hash
    // inside UniswapV2Library.pairFor() matches the actually deployed pair bytecode.
    function _deployFromArtifact(string memory artifactPath, bytes memory constructorArgs)
        internal
        returns (address deployed)
    {
        bytes memory bytecode = vm.parseJsonBytes(vm.readFile(artifactPath), ".bytecode");
        bytes memory creationCode = abi.encodePacked(bytecode, constructorArgs);
        assembly {
            deployed := create(0, add(creationCode, 0x20), mload(creationCode))
        }
        require(deployed != address(0) && deployed.code.length > 0, "artifact deploy failed");
    }

    function setUp() public {
        // Deploy Uniswap V2 from official artifacts (init code hash must match)
        uniswapFactory = _deployFromArtifact(
            "script/artifacts/UniswapV2Factory.json",
            abi.encode(address(0))
        );
        // WETH is unused (no ETH liquidity paths) — any address works
        uniswapRouter = _deployFromArtifact(
            "script/artifacts/UniswapV2Router02.json",
            abi.encode(uniswapFactory, address(1))
        );

        // Deploy IDRX mock
        vm.prank(admin);
        idrxToken = new IDRX(admin);

        // Deploy PulsarProtocol behind UUPS proxy
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
                (admin, uniswapRouter, address(idrxToken), custodians, admin)
            )
        );
        protocol = PulsarProtocol(address(proxy));

        // Fund cust1 and trader with IDRX
        vm.startPrank(admin);
        idrxToken.mint(cust1,  CUST1_IDRX_BALANCE);
        idrxToken.mint(trader, 50_000_000); // 500 000.00 IDRX for swap tests
        vm.stopPrank();

        // KYC approve trader
        vm.prank(admin);
        protocol.approveKYC(trader);
    }

    // ─── Helpers ──────────────────────────────────────────────────────────────

    /// Submit a requestMint from cust1 and return proposalId.
    function _requestMint(PulsarProtocol.MintDestination dest) internal returns (uint256 proposalId) {
        vm.prank(cust1);
        proposalId = protocol.requestMint("BUMIP", "Pulsar Bumi Resources", "BUMI", TOKEN_AMOUNT, IDRX_AMOUNT, ATTEST, dest);
    }

    /// Reach threshold (cust2 + cust3 approve) so cust1 can executeMint.
    function _reachThreshold(uint256 proposalId) internal {
        vm.prank(cust2);
        protocol.approveMint(proposalId);
        vm.prank(cust3);
        protocol.approveMint(proposalId);
    }

    /// Full mint flow to LiquidityPool: request → approve × 2 → executeMint.
    /// cust1 must have approved protocol for IDRX before calling this.
    function _fullMintToPool(uint256 existingProposalId) internal {
        _reachThreshold(existingProposalId);
        vm.prank(cust1);
        protocol.executeMint(existingProposalId);
    }

    // ─── requestMint ──────────────────────────────────────────────────────────

    function test_requestMint_liquidityPool_doesNotPullIDRX() public {
        uint256 balanceBefore = idrxToken.balanceOf(cust1);

        _requestMint(PulsarProtocol.MintDestination.LiquidityPool);

        assertEq(idrxToken.balanceOf(cust1), balanceBefore, "requestMint must not pull IDRX from requester");
    }

    function test_requestMint_operatorWallet_doesNotPullIDRX() public {
        uint256 balanceBefore = idrxToken.balanceOf(cust1);

        _requestMint(PulsarProtocol.MintDestination.OperatorWallet);

        assertEq(idrxToken.balanceOf(cust1), balanceBefore, "requestMint must not pull IDRX for wallet destination");
    }

    function test_requestMint_setsProposalFields() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);

        (
            string memory ticker,
            ,
            ,
            uint256 tokenAmount,
            uint256 idrxAmount,
            ,
            PulsarProtocol.MintDestination dest,
            address requester,
            uint8 approvalCount,
            bool executed,
            ,
        ) = protocol.proposals(proposalId);

        assertEq(ticker,              "BUMIP");
        assertEq(tokenAmount,         TOKEN_AMOUNT);
        assertEq(idrxAmount,          IDRX_AMOUNT);
        assertEq(uint8(dest),         uint8(PulsarProtocol.MintDestination.LiquidityPool));
        assertEq(requester,           cust1);
        assertEq(approvalCount,       1);
        assertFalse(executed);
    }

    function test_requestMint_preventsDoublePending() public {
        _requestMint(PulsarProtocol.MintDestination.LiquidityPool);

        vm.expectRevert(abi.encodeWithSelector(MintRequestPending.selector, "BUMIP"));
        vm.prank(cust1);
        protocol.requestMint("BUMIP", "Pulsar Bumi Resources", "BUMI", TOKEN_AMOUNT, IDRX_AMOUNT, ATTEST,
            PulsarProtocol.MintDestination.LiquidityPool);
    }

    // ─── approveMint ──────────────────────────────────────────────────────────

    function test_approveMint_incrementsCount() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);

        vm.prank(cust2);
        protocol.approveMint(proposalId);

        (,,,,,,,, uint8 approvalCount,,, ) = protocol.proposals(proposalId);
        assertEq(approvalCount, 2);
    }

    function test_approveMint_rejectsDouble() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);

        vm.prank(cust2);
        protocol.approveMint(proposalId);

        vm.expectRevert(abi.encodeWithSelector(AlreadyApproved.selector, proposalId, cust2));
        vm.prank(cust2);
        protocol.approveMint(proposalId);
    }

    // ─── executeMint → OperatorWallet ─────────────────────────────────────────

    function test_executeMint_operatorWallet_mintsToRequester() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.OperatorWallet);
        _reachThreshold(proposalId);

        vm.prank(cust1);
        protocol.executeMint(proposalId);

        address stockAddress = protocol.stocks("BUMIP");
        assertFalse(stockAddress == address(0), "stock contract must be deployed");
        assertEq(PulsarStock(stockAddress).balanceOf(cust1), TOKEN_AMOUNT, "tokens must go to requester");
    }

    function test_executeMint_operatorWallet_doesNotTouchPool() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.OperatorWallet);
        _reachThreshold(proposalId);

        vm.prank(cust1);
        protocol.executeMint(proposalId);

        address stockAddress = protocol.stocks("BUMIP");
        address pair = IUniswapV2Factory(uniswapFactory).getPair(stockAddress, address(idrxToken));
        assertEq(pair, address(0), "no pool must be created for wallet destination");
    }

    // ─── executeMint → LiquidityPool ──────────────────────────────────────────

    function test_executeMint_liquidityPool_pullsIDRXAtExecution() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);
        _reachThreshold(proposalId);

        // IDRX balance before execution must be unchanged from after requestMint
        uint256 balanceBeforeExec = idrxToken.balanceOf(cust1);

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId);
        vm.stopPrank();

        uint256 balanceAfterExec = idrxToken.balanceOf(cust1);
        // Uniswap may not consume the exact amount (ratio might differ); assert at most IDRX_AMOUNT was pulled
        assertLe(
            balanceBeforeExec - balanceAfterExec,
            IDRX_AMOUNT,
            "executeMint must pull at most IDRX_AMOUNT from requester"
        );
        assertLt(balanceAfterExec, balanceBeforeExec, "executeMint must pull some IDRX from requester");
    }

    function test_executeMint_liquidityPool_createsPool() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);
        _reachThreshold(proposalId);

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId);
        vm.stopPrank();

        address stockAddress = protocol.stocks("BUMIP");
        address pair = IUniswapV2Factory(uniswapFactory).getPair(stockAddress, address(idrxToken));
        assertFalse(pair == address(0), "Uniswap V2 pool must exist after executeMint to LP");
    }

    function test_executeMint_liquidityPool_emitsPoolCreated() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);
        _reachThreshold(proposalId);

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        vm.expectEmit(true, false, false, false, address(protocol));
        emit PulsarProtocol.PoolCreated("BUMIP", 0, 0, 0);
        protocol.executeMint(proposalId);
        vm.stopPrank();
    }

    function test_executeMint_liquidityPool_secondMintEmitsLiquidityAdded() public {
        // First mint creates the pool
        uint256 firstId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);
        _reachThreshold(firstId);
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(firstId);
        vm.stopPrank();

        // Second mint adds to the existing pool
        vm.prank(cust2);
        uint256 secondId = protocol.requestMint(
            "BUMIP", "Pulsar Bumi Resources", "BUMI", TOKEN_AMOUNT, IDRX_AMOUNT, ATTEST,
            PulsarProtocol.MintDestination.LiquidityPool
        );
        vm.prank(cust1);
        protocol.approveMint(secondId);
        vm.prank(cust3);
        protocol.approveMint(secondId);

        vm.startPrank(cust2);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        // Ensure cust2 has enough IDRX
        vm.stopPrank();
        vm.prank(admin);
        idrxToken.mint(cust2, IDRX_AMOUNT);

        vm.startPrank(cust2);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        vm.expectEmit(true, false, false, false, address(protocol));
        emit PulsarProtocol.LiquidityAdded("BUMIP", 0, 0, 0);
        protocol.executeMint(secondId);
        vm.stopPrank();
    }

    function test_executeMint_liquidityPool_revertsWithoutIDRXApproval() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);
        _reachThreshold(proposalId);

        // No idrxToken.approve() — must revert
        vm.prank(cust1);
        vm.expectRevert();
        protocol.executeMint(proposalId);
    }

    function test_executeMint_revertsIfNotRequester() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);
        _reachThreshold(proposalId);

        vm.expectRevert(abi.encodeWithSelector(NotRequester.selector, proposalId, cust2));
        vm.prank(cust2);
        protocol.executeMint(proposalId);
    }

    function test_executeMint_revertsIfThresholdNotMet() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);
        // Only 1 approval (the requester's own) — threshold is 3

        vm.prank(cust1);
        vm.expectRevert(abi.encodeWithSelector(ThresholdNotMet.selector, proposalId, 1, 3));
        protocol.executeMint(proposalId);
    }

    function test_executeMint_revertsIfAlreadyExecuted() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.OperatorWallet);
        _reachThreshold(proposalId);

        vm.prank(cust1);
        protocol.executeMint(proposalId);

        vm.expectRevert(abi.encodeWithSelector(ProposalAlreadyExecuted.selector, proposalId));
        vm.prank(cust1);
        protocol.executeMint(proposalId);
    }

    // ─── rejectMint / executeRejectMint ──────────────────────────────────────

    function test_executeRejectMint_noFunding_noRefund() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);

        uint256 balanceBefore = idrxToken.balanceOf(cust1);

        vm.prank(cust1);
        protocol.rejectMint(proposalId);
        vm.prank(cust2);
        protocol.rejectMint(proposalId);
        vm.prank(cust3);
        protocol.rejectMint(proposalId);

        vm.prank(cust1); // cust1 is the rejectInitiator
        protocol.executeRejectMint(proposalId);

        assertEq(idrxToken.balanceOf(cust1), balanceBefore, "no IDRX was locked so balance must be unchanged");
        assertEq(protocol.mintLiquidityFunding(proposalId), 0);
    }

    function test_executeRejectMint_legacyFunding_refundsRequester() public {
        // Simulate a legacy pre-upgrade proposal that had IDRX pre-funded via fundMintLiquidity.
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);

        // Use the (now-deprecated) fundMintLiquidity to simulate legacy pre-upgrade state
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
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);
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
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);

        vm.prank(cust1);
        protocol.rejectMint(proposalId);
        vm.prank(cust2);
        protocol.rejectMint(proposalId);
        // Only 2 reject votes — threshold is 3

        vm.expectRevert(abi.encodeWithSelector(ThresholdNotMet.selector, proposalId, 2, 3));
        vm.prank(cust1);
        protocol.executeRejectMint(proposalId);
    }

    // ─── Legacy: executeMint with pre-funded IDRX ────────────────────────────

    function test_executeMint_liquidityPool_legacyFunding_usesExistingIDRX() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);

        // Simulate legacy state: full amount already in mintLiquidityFunding
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.fundMintLiquidity(proposalId, IDRX_AMOUNT);
        vm.stopPrank();

        _reachThreshold(proposalId);

        uint256 cust1BalanceBefore = idrxToken.balanceOf(cust1);

        // No additional approve needed — IDRX already funded
        vm.prank(cust1);
        protocol.executeMint(proposalId);

        // cust1's balance should NOT decrease further (all IDRX came from legacy funding)
        assertEq(
            idrxToken.balanceOf(cust1),
            cust1BalanceBefore,
            "legacy fully-funded proposal must not pull additional IDRX at executeMint"
        );

        // Pool must exist
        address stockAddress = protocol.stocks("BUMIP");
        address pair = IUniswapV2Factory(uniswapFactory).getPair(stockAddress, address(idrxToken));
        assertFalse(pair == address(0), "pool must be created");

        // Funding cleared
        assertEq(protocol.mintLiquidityFunding(proposalId), 0);
    }

    function test_executeMint_liquidityPool_legacyPartialFunding_pullsShortfall() public {
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);

        uint256 partialFund = IDRX_AMOUNT / 2;

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), partialFund);
        protocol.fundMintLiquidity(proposalId, partialFund);
        vm.stopPrank();

        _reachThreshold(proposalId);

        uint256 balanceBefore = idrxToken.balanceOf(cust1);

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId);
        vm.stopPrank();

        uint256 pulled = balanceBefore - idrxToken.balanceOf(cust1);
        // Should only pull the shortfall (at most IDRX_AMOUNT - partialFund)
        assertLe(pulled, IDRX_AMOUNT - partialFund, "must not pull more than the shortfall");

        address stockAddress = protocol.stocks("BUMIP");
        address pair = IUniswapV2Factory(uniswapFactory).getPair(stockAddress, address(idrxToken));
        assertFalse(pair == address(0), "pool must be created");
    }

    // ─── Swap ─────────────────────────────────────────────────────────────────

    function test_swap_buyStock_afterPoolCreated() public {
        // First create the pool
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);
        _reachThreshold(proposalId);
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId);
        vm.stopPrank();

        address stockAddress = protocol.stocks("BUMIP");
        uint256 swapIn = 250_000; // 2500.00 IDRX

        vm.startPrank(trader);
        idrxToken.approve(address(protocol), swapIn);
        protocol.swap("BUMIP", swapIn, 0, true);
        vm.stopPrank();

        assertGt(PulsarStock(stockAddress).balanceOf(trader), 0, "trader must receive pStock tokens");
    }

    function test_swap_sellStock_afterPoolCreated() public {
        // Create pool
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);
        _reachThreshold(proposalId);
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId);
        vm.stopPrank();

        // Buy first
        uint256 swapIn = 250_000;
        vm.startPrank(trader);
        idrxToken.approve(address(protocol), swapIn);
        protocol.swap("BUMIP", swapIn, 0, true);
        vm.stopPrank();

        address stockAddress = protocol.stocks("BUMIP");
        uint256 stockBalance = PulsarStock(stockAddress).balanceOf(trader);
        assertGt(stockBalance, 0);

        // Sell back
        uint256 idrxBefore = idrxToken.balanceOf(trader);
        vm.startPrank(trader);
        PulsarStock(stockAddress).approve(address(protocol), stockBalance);
        protocol.swap("BUMIP", stockBalance, 0, false);
        vm.stopPrank();

        assertGt(idrxToken.balanceOf(trader), idrxBefore, "trader must receive IDRX after selling");
    }

    function test_swap_revertsForUnknownTicker() public {
        vm.prank(trader);
        vm.expectRevert(abi.encodeWithSelector(StockNotFound.selector, "XXXX"));
        protocol.swap("XXXX", 1000, 0, true);
    }

    // ─── KYC ──────────────────────────────────────────────────────────────────

    function test_requestRedeem_revertsWithoutKYC() public {
        // Create pool + buy some stock so user has tokens
        uint256 proposalId = _requestMint(PulsarProtocol.MintDestination.LiquidityPool);
        _reachThreshold(proposalId);
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId);
        vm.stopPrank();

        address noKycUser = makeAddr("noKyc");
        vm.prank(admin);
        idrxToken.mint(noKycUser, 100_000);

        address stockAddress = protocol.stocks("BUMIP");

        vm.startPrank(noKycUser);
        idrxToken.approve(address(protocol), 10_000);
        PulsarStock(stockAddress).approve(address(protocol), 1e18);
        vm.expectRevert(abi.encodeWithSelector(KYCRequired.selector, noKycUser));
        protocol.requestRedeem("BUMIP", 1e18);
        vm.stopPrank();
    }

    // ─── Access control ───────────────────────────────────────────────────────

    function test_nonCustodian_cannotRequestMint() public {
        vm.prank(trader);
        vm.expectRevert();
        protocol.requestMint("BUMIP", "Pulsar Bumi Resources", "BUMI", TOKEN_AMOUNT, IDRX_AMOUNT, ATTEST,
            PulsarProtocol.MintDestination.LiquidityPool);
    }

    function test_nonAdmin_cannotApproveKYC() public {
        vm.prank(cust1);
        vm.expectRevert();
        protocol.approveKYC(trader);
    }
}

// Bring custom error selectors into scope for vm.expectRevert
error MintRequestPending(string ticker);
error StockNotFound(string ticker);
error AlreadyApproved(uint256 proposalId, address custodian);
error NotRequester(uint256 proposalId, address caller);
error ThresholdNotMet(uint256 proposalId, uint8 current, uint8 required);
error ProposalAlreadyExecuted(uint256 proposalId);
error NotMintRejectInitiator(uint256 proposalId, address caller);
error KYCRequired(address wallet);

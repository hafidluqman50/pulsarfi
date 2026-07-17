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

        // KYC approve trader (approveKYC requires CUSTODIAN_ROLE, not DEFAULT_ADMIN_ROLE)
        vm.prank(cust1);
        protocol.approveKYC(trader);
    }

    // ─── Helpers ──────────────────────────────────────────────────────────────

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

    /// Full mint flow: request → approve × 2 → executeMint.
    /// cust1 must have approved protocol for IDRX before calling this.
    function _fullMintToPool(uint256 existingProposalId) internal {
        _reachThreshold(existingProposalId);
        vm.prank(cust1);
        protocol.executeMint(existingProposalId);
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

    // ─── executeMint ──────────────────────────────────────────────────────────

    function test_executeMint_pullsIDRXAtExecution() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);

        uint256 balanceBeforeExec = idrxToken.balanceOf(cust1);

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId);
        vm.stopPrank();

        uint256 balanceAfterExec = idrxToken.balanceOf(cust1);
        assertLe(
            balanceBeforeExec - balanceAfterExec,
            IDRX_AMOUNT,
            "executeMint must pull at most IDRX_AMOUNT from requester"
        );
        assertLt(balanceAfterExec, balanceBeforeExec, "executeMint must pull some IDRX from requester");
    }

    function test_executeMint_createsPool() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId);
        vm.stopPrank();

        address stockAddress = protocol.stocks("BUMIP");
        address pair = IUniswapV2Factory(uniswapFactory).getPair(stockAddress, address(idrxToken));
        assertFalse(pair == address(0), "Uniswap V2 pool must exist after executeMint");
    }

    function test_executeMint_emitsPoolCreated() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        vm.expectEmit(true, false, false, false, address(protocol));
        emit PulsarProtocol.PoolCreated("BUMIP", 0, 0, 0);
        protocol.executeMint(proposalId);
        vm.stopPrank();
    }

    function test_executeMint_secondMintEmitsLiquidityAdded() public {
        // First mint creates the pool
        uint256 firstId = _requestMint();
        _reachThreshold(firstId);
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(firstId);
        vm.stopPrank();

        // Second mint adds to the existing pool
        vm.prank(cust2);
        uint256 secondId = protocol.requestMint(
            "BUMIP", "Pulsar Bumi Resources", "BUMI", TOKEN_AMOUNT, IDRX_AMOUNT, ATTEST
        );
        vm.prank(cust1);
        protocol.approveMint(secondId);
        vm.prank(cust3);
        protocol.approveMint(secondId);

        vm.prank(admin);
        idrxToken.mint(cust2, IDRX_AMOUNT);

        vm.startPrank(cust2);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        vm.expectEmit(true, false, false, false, address(protocol));
        emit PulsarProtocol.LiquidityAdded("BUMIP", 0, 0, 0);
        protocol.executeMint(secondId);
        vm.stopPrank();
    }

    function test_executeMint_revertsWithoutIDRXApproval() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);

        // No idrxToken.approve() — must revert
        vm.prank(cust1);
        vm.expectRevert();
        protocol.executeMint(proposalId);
    }

    function test_executeMint_revertsIfNotRequester() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);

        vm.expectRevert(abi.encodeWithSelector(NotRequester.selector, proposalId, cust2));
        vm.prank(cust2);
        protocol.executeMint(proposalId);
    }

    function test_executeMint_revertsIfThresholdNotMet() public {
        uint256 proposalId = _requestMint();

        vm.prank(cust1);
        vm.expectRevert(abi.encodeWithSelector(ThresholdNotMet.selector, proposalId, 1, 3));
        protocol.executeMint(proposalId);
    }

    function test_executeMint_revertsIfAlreadyExecuted() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId);
        vm.stopPrank();

        vm.expectRevert(abi.encodeWithSelector(ProposalAlreadyExecuted.selector, proposalId));
        vm.prank(cust1);
        protocol.executeMint(proposalId);
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

        vm.prank(cust1); // cust1 is the rejectInitiator
        protocol.executeRejectMint(proposalId);

        assertEq(idrxToken.balanceOf(cust1), balanceBefore, "no IDRX was locked so balance must be unchanged");
        assertEq(protocol.mintLiquidityFunding(proposalId), 0);
    }

    function test_executeRejectMint_legacyFunding_refundsRequester() public {
        // Simulate a legacy pre-upgrade proposal that had IDRX pre-funded via fundMintLiquidity.
        uint256 proposalId = _requestMint();

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
        // Only 2 reject votes — threshold is 3

        vm.expectRevert(abi.encodeWithSelector(ThresholdNotMet.selector, proposalId, 2, 3));
        vm.prank(cust1);
        protocol.executeRejectMint(proposalId);
    }

    // ─── Legacy: executeMint with pre-funded IDRX ────────────────────────────

    function test_executeMint_legacyFunding_usesExistingIDRX() public {
        uint256 proposalId = _requestMint();

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

        address stockAddress = protocol.stocks("BUMIP");
        address pair = IUniswapV2Factory(uniswapFactory).getPair(stockAddress, address(idrxToken));
        assertFalse(pair == address(0), "pool must be created");
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
        protocol.executeMint(proposalId);
        vm.stopPrank();

        uint256 pulled = balanceBefore - idrxToken.balanceOf(cust1);
        assertLe(pulled, IDRX_AMOUNT - partialFund, "must not pull more than the shortfall");

        address stockAddress = protocol.stocks("BUMIP");
        address pair = IUniswapV2Factory(uniswapFactory).getPair(stockAddress, address(idrxToken));
        assertFalse(pair == address(0), "pool must be created");
    }

    // ─── Swap ─────────────────────────────────────────────────────────────────

    function test_swap_buyStock_afterPoolCreated() public {
        uint256 proposalId = _requestMint();
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
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId);
        vm.stopPrank();

        uint256 swapIn = 250_000;
        vm.startPrank(trader);
        idrxToken.approve(address(protocol), swapIn);
        protocol.swap("BUMIP", swapIn, 0, true);
        vm.stopPrank();

        address stockAddress = protocol.stocks("BUMIP");
        uint256 stockBalance = PulsarStock(stockAddress).balanceOf(trader);
        assertGt(stockBalance, 0);

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
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId);
        vm.stopPrank();

        address noKycUser = makeAddr("noKyc");
        vm.prank(admin);
        idrxToken.mint(noKycUser, 100_000);

        address stockAddress = protocol.stocks("BUMIP");
        vm.prank(noKycUser);
        PulsarStock(stockAddress).approve(address(protocol), 1e18);

        vm.prank(noKycUser);
        vm.expectRevert(abi.encodeWithSelector(KYCRequired.selector, noKycUser));
        protocol.requestRedeem("BUMIP", 1e18);
    }

    /// requestRedeem is permissionless: any KYC-approved wallet can call it directly,
    /// it is not custodian-gated.
    function test_requestRedeem_permissionlessForNonCustodianKycUser() public {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId);
        vm.stopPrank();

        address stockAddress = protocol.stocks("BUMIP");
        address kycUser = makeAddr("kycUser");
        vm.prank(admin);
        idrxToken.mint(kycUser, 100_000);
        vm.prank(cust1);
        protocol.approveKYC(kycUser);

        vm.startPrank(kycUser);
        idrxToken.approve(address(protocol), 100_000);
        protocol.swap("BUMIP", 100_000, 0, true);
        uint256 stockBalance = PulsarStock(stockAddress).balanceOf(kycUser);
        PulsarStock(stockAddress).approve(address(protocol), stockBalance);
        protocol.requestRedeem("BUMIP", stockBalance);
        vm.stopPrank();

        assertEq(protocol.redeemRequestCount(), 1, "non-custodian KYC user must be able to requestRedeem directly");
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

    // ─── Swap fee ─────────────────────────────────────────────────────────────

    function _mintAndPool() internal returns (address stockAddress) {
        uint256 proposalId = _requestMint();
        _reachThreshold(proposalId);
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(proposalId);
        vm.stopPrank();
        stockAddress = protocol.stocks("BUMIP");
    }

    function test_swap_buyStock_deductsProtocolFeeFromInput() public {
        _mintAndPool();

        vm.prank(admin);
        protocol.setSwapFeeBps(20); // 0.2%

        uint256 swapIn = 250_000; // 2 500.00 IDRX
        uint256 expectedFee = (swapIn * 20) / 10_000;

        vm.startPrank(trader);
        idrxToken.approve(address(protocol), swapIn);
        protocol.swap("BUMIP", swapIn, 0, true);
        vm.stopPrank();

        assertEq(protocol.accumulatedFees(), expectedFee, "buy-side protocol fee must be recorded exactly");
    }

    function test_swap_sellStock_deductsProtocolFeeFromOutput() public {
        address stockAddress = _mintAndPool();

        // Buy first (no fee yet) so trader holds stock to sell.
        uint256 swapIn = 250_000;
        vm.startPrank(trader);
        idrxToken.approve(address(protocol), swapIn);
        protocol.swap("BUMIP", swapIn, 0, true);
        vm.stopPrank();

        uint256 stockBalance = PulsarStock(stockAddress).balanceOf(trader);

        vm.prank(admin);
        protocol.setSwapFeeBps(20); // 0.2%

        uint256 idrxBefore = idrxToken.balanceOf(trader);

        vm.startPrank(trader);
        PulsarStock(stockAddress).approve(address(protocol), stockBalance);
        protocol.swap("BUMIP", stockBalance, 0, false);
        vm.stopPrank();

        uint256 received = idrxToken.balanceOf(trader) - idrxBefore;

        assertGt(protocol.accumulatedFees(), 0, "sell-side protocol fee must be recorded");
        assertGt(received, 0, "trader must still receive net IDRX after fee");
    }

    function test_swap_zeroFeeBps_doesNotTouchContractBalance() public {
        address stockAddress = _mintAndPool();

        // swapFeeBps stays at its default (0) — sell path must not try to
        // forward funds the contract never received.
        uint256 swapIn = 250_000;
        vm.startPrank(trader);
        idrxToken.approve(address(protocol), swapIn);
        protocol.swap("BUMIP", swapIn, 0, true);
        vm.stopPrank();

        uint256 stockBalance = PulsarStock(stockAddress).balanceOf(trader);
        uint256 idrxBefore = idrxToken.balanceOf(trader);

        vm.startPrank(trader);
        PulsarStock(stockAddress).approve(address(protocol), stockBalance);
        protocol.swap("BUMIP", stockBalance, 0, false);
        vm.stopPrank();

        assertGt(idrxToken.balanceOf(trader), idrxBefore, "trader must receive IDRX with no protocol fee");
        assertEq(protocol.accumulatedFees(), 0, "no fee should accrue when swapFeeBps is 0");
    }

    // ─── Fuzz: swap fee ───────────────────────────────────────────────────────

    function testFuzz_swap_buyStock_feeMatchesFormula(uint256 feeBps, uint256 swapIn) public {
        feeBps = bound(feeBps, 0, 1000); // max 10%, contract-enforced cap
        swapIn = bound(swapIn, 10_000, 40_000_000); // 100.00 - 400 000.00 IDRX, within trader's balance

        _mintAndPool();

        vm.prank(admin);
        protocol.setSwapFeeBps(feeBps);

        uint256 expectedFee = (swapIn * feeBps) / 10_000;

        vm.startPrank(trader);
        idrxToken.approve(address(protocol), swapIn);
        protocol.swap("BUMIP", swapIn, 0, true);
        vm.stopPrank();

        assertEq(protocol.accumulatedFees(), expectedFee, "buy-side fee must match bps formula for any feeBps/amount");
        assertLe(expectedFee, swapIn, "fee must never exceed the swapped-in amount");
    }

    function testFuzz_swap_sellStock_feeMatchesFormula(uint256 feeBps) public {
        feeBps = bound(feeBps, 0, 1000); // max 10%, contract-enforced cap

        address stockAddress = _mintAndPool();

        uint256 swapIn = 250_000;
        vm.startPrank(trader);
        idrxToken.approve(address(protocol), swapIn);
        protocol.swap("BUMIP", swapIn, 0, true);
        vm.stopPrank();

        uint256 stockBalance = PulsarStock(stockAddress).balanceOf(trader);

        vm.prank(admin);
        protocol.setSwapFeeBps(feeBps);

        uint256 idrxBefore = idrxToken.balanceOf(trader);

        vm.startPrank(trader);
        PulsarStock(stockAddress).approve(address(protocol), stockBalance);
        protocol.swap("BUMIP", stockBalance, 0, false);
        vm.stopPrank();

        uint256 received = idrxToken.balanceOf(trader) - idrxBefore;
        uint256 feeCollected = protocol.accumulatedFees();
        uint256 rawOut = received + feeCollected;

        assertEq(feeCollected, (rawOut * feeBps) / 10_000, "sell-side fee must match bps formula for any feeBps");
        assertLe(feeCollected, rawOut, "fee must never exceed the raw swap output");
    }

    // ─── Fuzz: distributeFees conservation ─────────────────────────────────────

    function testFuzz_distributeFees_conservesTotalAcrossFeeRates(uint256 feeBps) public {
        feeBps = bound(feeBps, 1, 1000); // >0 so fees actually accrue

        _mintAndPool();
        vm.prank(admin);
        protocol.setSwapFeeBps(feeBps);

        uint256 swapIn = 250_000;
        vm.startPrank(trader);
        idrxToken.approve(address(protocol), swapIn);
        protocol.swap("BUMIP", swapIn, 0, true);
        vm.stopPrank();

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
            "distribution must conserve the full fee total exactly, no dust lost or created, for any feeBps"
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

        uint256 swapIn = 250_000;
        vm.startPrank(trader);
        idrxToken.approve(address(protocol), swapIn);
        protocol.swap("BUMIP", swapIn, 0, true);
        vm.stopPrank();

        uint256 fees = protocol.accumulatedFees();
        assertGt(fees, 0);

        // Active custodians at this point: cust1 (requester) + cust2, cust3 (approvers) = 3.
        uint256 expectedTreasuryShare = (fees * 30) / 100;
        uint256 expectedCustodianPool = fees - expectedTreasuryShare;
        uint256 expectedPerCustodian = expectedCustodianPool / 3;
        uint256 expectedRemainder = expectedCustodianPool - (expectedPerCustodian * 3);

        // treasury == admin per setUp's initialize(..., treasury_: admin)
        uint256 treasuryBefore = idrxToken.balanceOf(admin);
        uint256 cust1Before = idrxToken.balanceOf(cust1);

        protocol.distributeFees();

        assertEq(protocol.accumulatedFees(), 0, "accumulatedFees must reset after distribution");
        uint256 treasuryGain = idrxToken.balanceOf(admin) - treasuryBefore;
        uint256 cust1Gain = idrxToken.balanceOf(cust1) - cust1Before;
        assertEq(treasuryGain, expectedTreasuryShare + expectedRemainder, "treasury gets 30% plus rounding remainder");
        assertEq(cust1Gain, expectedPerCustodian, "active custodian (cust1) gets an equal 1/3 share");
    }

    /// The core safety invariant behind accumulatedFees: a redeem fee still
    /// locked as pending escrow (not yet executed/rejected) must never be
    /// swept by distributeFees, since it may still need to be refunded.
    function test_distributeFees_doesNotTouchPendingRedeemEscrow() public {
        address stockAddress = _mintAndPool();

        vm.prank(admin);
        protocol.setRedeemFeeBps(100); // 1% exit fee

        uint256 swapIn = 250_000;
        vm.startPrank(trader);
        idrxToken.approve(address(protocol), swapIn);
        protocol.swap("BUMIP", swapIn, 0, true);
        vm.stopPrank();

        uint256 stockBalance = PulsarStock(stockAddress).balanceOf(trader);

        // trader requests redeem — locks stock + redeem fee IDRX as PENDING escrow,
        // not yet added to accumulatedFees (only happens at executeRedeem).
        vm.startPrank(trader);
        PulsarStock(stockAddress).approve(address(protocol), stockBalance);
        idrxToken.approve(address(protocol), type(uint256).max);
        protocol.requestRedeem("BUMIP", stockBalance);
        vm.stopPrank();

        uint256 requestId = protocol.redeemRequestCount() - 1;
        (,, , uint256 lockedFee,,,,,,) = protocol.redeemRequests(requestId);
        assertGt(lockedFee, 0, "redeem fee must be locked pending a decision");
        assertEq(protocol.accumulatedFees(), 0, "pending redeem fee must NOT be in accumulatedFees yet");

        // Separately, generate real confirmed revenue via a swap fee.
        vm.prank(admin);
        protocol.setSwapFeeBps(20); // 0.2%
        vm.startPrank(trader);
        idrxToken.approve(address(protocol), swapIn);
        protocol.swap("BUMIP", swapIn, 0, true);
        vm.stopPrank();

        uint256 confirmedFees = protocol.accumulatedFees();
        assertGt(confirmedFees, 0, "swap fee must have accrued separately from the pending redeem escrow");

        // distributeFees must only touch confirmed accumulatedFees, never the pending escrow.
        protocol.distributeFees();
        assertEq(protocol.accumulatedFees(), 0, "confirmed fees must be distributed");

        // Reject the pending redeem — this must still be able to refund the FULL locked
        // fee. If distributeFees had swept the escrow, this would revert with
        // ERC20InsufficientBalance.
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
        // Deploy a fresh protocol with no custodians ever having requested/approved a mint,
        // so _activeCustodians is empty even though accumulatedFees could theoretically be nonzero.
        address[] memory noCustodians = new address[](0);
        PulsarProtocol freshImpl = new PulsarProtocol();
        ERC1967Proxy freshProxy = new ERC1967Proxy(
            address(freshImpl),
            abi.encodeCall(PulsarProtocol.initialize, (admin, uniswapRouter, address(idrxToken), noCustodians, admin))
        );
        PulsarProtocol fresh = PulsarProtocol(address(freshProxy));

        vm.expectRevert(NoActiveCustodians.selector);
        fresh.distributeFees();
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
error BelowDistributionThreshold(uint256 balance, uint256 threshold);
error NoActiveCustodians();

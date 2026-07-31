// SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

import {Test} from "forge-std/Test.sol";

import {IPoolManager} from "@uniswap/v4-core/src/interfaces/IPoolManager.sol";
import {PoolSwapTest} from "@uniswap/v4-core/src/test/PoolSwapTest.sol";
import {PoolModifyLiquidityTest} from "@uniswap/v4-core/src/test/PoolModifyLiquidityTest.sol";
import {Hooks} from "@uniswap/v4-core/src/libraries/Hooks.sol";
import {TickMath} from "@uniswap/v4-core/src/libraries/TickMath.sol";
import {PoolKey} from "@uniswap/v4-core/src/types/PoolKey.sol";
import {PoolId} from "@uniswap/v4-core/src/types/PoolId.sol";
import {Currency} from "@uniswap/v4-core/src/types/Currency.sol";
import {IHooks} from "@uniswap/v4-core/src/interfaces/IHooks.sol";
import {BalanceDelta} from "@uniswap/v4-core/src/types/BalanceDelta.sol";
import {SwapParams, ModifyLiquidityParams} from "@uniswap/v4-core/src/types/PoolOperation.sol";
import {HookMiner} from "@uniswap/v4-periphery/src/utils/HookMiner.sol";

import {PulsarSwapHook} from "../../src/v4/PulsarSwapHook.sol";

/// @dev Minimal ERC-20 (mintable) so the test carries no external token dependency.
contract MiniERC20 {
    string public name;
    string public symbol;
    uint8 public decimals;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    constructor(string memory n, string memory s, uint8 d) {
        name = n;
        symbol = s;
        decimals = d;
    }

    function mint(address to, uint256 amount) external {
        balanceOf[to] += amount;
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        return true;
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        uint256 a = allowance[from][msg.sender];
        if (a != type(uint256).max) allowance[from][msg.sender] = a - amount;
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        return true;
    }
}

contract MockFeeConfig {
    uint256 public swapFeeBps;

    constructor(uint256 bps) {
        swapFeeBps = bps;
    }

    function setSwapFeeBps(uint256 bps) external {
        swapFeeBps = bps;
    }
}

/// @notice Fork integration test against the REAL Uniswap V4 PoolManager on
///         Arbitrum Sepolia. Proves the protocol swap fee is taken in IDRX on
///         BOTH directions and that the hook accrues zero stock-token claims
///         (strict IDRX-only accounting). Runs only when RPC_URL is set.
contract PulsarSwapHookForkTest is Test {
    // Canonical Uniswap V4 PoolManager on Arbitrum Sepolia (verified on-chain).
    address constant POOL_MANAGER = 0xFB3e0C6F74eB1a21CC1Da29aeC80D2Dfe6C9a317;

    uint256 constant FEE_BPS = 20; // 0.2%
    uint24 constant LP_FEE = 0; // isolate the hook fee for clean assertions
    int24 constant TICK_SPACING = 60;
    uint160 constant SQRT_PRICE_1_1 = 79228162514264337593543950336;

    IPoolManager pm;
    PoolSwapTest swapRouter;
    PoolModifyLiquidityTest liqRouter;
    MockFeeConfig feeConfig;
    PulsarSwapHook hook;

    MiniERC20 idrxToken;
    MiniERC20 stockToken;
    Currency idrx;
    Currency stock;
    PoolKey poolKey;

    address trader = makeAddr("trader");

    function setUp() public {
        string memory rpc = vm.envOr("RPC_URL", string(""));
        if (bytes(rpc).length == 0) return;
        vm.createSelectFork(rpc);

        pm = IPoolManager(POOL_MANAGER);
        swapRouter = new PoolSwapTest(pm);
        liqRouter = new PoolModifyLiquidityTest(pm);

        idrxToken = new MiniERC20("IDRX", "IDRX", 2);
        stockToken = new MiniERC20("Pulsar BUMI", "BUMIP", 18);

        feeConfig = new MockFeeConfig(FEE_BPS);

        // Mine + deploy the hook at an address carrying the permission flags.
        uint160 flags = uint160(
            Hooks.BEFORE_SWAP_FLAG | Hooks.AFTER_SWAP_FLAG | Hooks.BEFORE_SWAP_RETURNS_DELTA_FLAG
                | Hooks.AFTER_SWAP_RETURNS_DELTA_FLAG
        );
        bytes memory args = abi.encode(address(pm), address(feeConfig), address(idrxToken), address(this), address(this));
        (address hookAddr, bytes32 salt) =
            HookMiner.find(address(this), flags, type(PulsarSwapHook).creationCode, args);
        hook = new PulsarSwapHook{salt: salt}(
            pm, address(feeConfig), address(idrxToken), address(this), address(this)
        );
        assertEq(address(hook), hookAddr);

        // Order currencies by address as V4 requires (currency0 < currency1).
        (Currency c0, Currency c1) = address(idrxToken) < address(stockToken)
            ? (Currency.wrap(address(idrxToken)), Currency.wrap(address(stockToken)))
            : (Currency.wrap(address(stockToken)), Currency.wrap(address(idrxToken)));
        idrx = Currency.wrap(address(idrxToken));
        stock = Currency.wrap(address(stockToken));

        poolKey = PoolKey({currency0: c0, currency1: c1, fee: LP_FEE, tickSpacing: TICK_SPACING, hooks: IHooks(address(hook))});
        pm.initialize(poolKey, SQRT_PRICE_1_1);
        hook.registerPool(poolKey, "BUMIP");

        // Seed liquidity (full range) from this contract.
        idrxToken.mint(address(this), 1e30);
        stockToken.mint(address(this), 1e30);
        idrxToken.approve(address(liqRouter), type(uint256).max);
        stockToken.approve(address(liqRouter), type(uint256).max);

        int24 tickLower = (TickMath.MIN_TICK / TICK_SPACING) * TICK_SPACING;
        int24 tickUpper = (TickMath.MAX_TICK / TICK_SPACING) * TICK_SPACING;
        liqRouter.modifyLiquidity(
            poolKey,
            ModifyLiquidityParams({tickLower: tickLower, tickUpper: tickUpper, liquidityDelta: 1e18, salt: bytes32(0)}),
            ""
        );

        // Fund trader + approvals.
        idrxToken.mint(trader, 1e24);
        stockToken.mint(trader, 1e24);
        vm.startPrank(trader);
        idrxToken.approve(address(swapRouter), type(uint256).max);
        stockToken.approve(address(swapRouter), type(uint256).max);
        vm.stopPrank();
    }

    function _idrxClaimBalance() internal view returns (uint256) {
        return pm.balanceOf(address(hook), idrx.toId());
    }

    function _stockClaimBalance() internal view returns (uint256) {
        return pm.balanceOf(address(hook), stock.toId());
    }

    function _isIdrxCurrency0() internal view returns (bool) {
        return address(idrxToken) < address(stockToken);
    }

    function test_buySide_takesFeeInIdrxFromInput() public {
        if (address(pm) == address(0)) return; // RPC not configured — skip

        uint256 amountIn = 1_000_000; // 10,000.00 IDRX (2 decimals)
        uint256 expectedFee = (amountIn * FEE_BPS) / 10_000;

        // buy = pay IDRX. zeroForOne true iff idrx is currency0.
        bool zeroForOne = _isIdrxCurrency0();

        uint256 feeBefore = _idrxClaimBalance();

        vm.prank(trader);
        swapRouter.swap(
            poolKey,
            SwapParams({
                zeroForOne: zeroForOne,
                amountSpecified: -int256(amountIn),
                sqrtPriceLimitX96: zeroForOne ? TickMath.MIN_SQRT_PRICE + 1 : TickMath.MAX_SQRT_PRICE - 1
            }),
            PoolSwapTest.TestSettings({takeClaims: false, settleUsingBurn: false}),
            ""
        );

        assertEq(_idrxClaimBalance() - feeBefore, expectedFee, "buy fee must equal bps of IDRX input");
        assertEq(_stockClaimBalance(), 0, "hook must never hold stock-token fees");
    }

    function test_sellSide_takesFeeInIdrxFromOutput() public {
        if (address(pm) == address(0)) return;

        uint256 amountIn = 1e18; // 1 stock token
        // sell = pay stock. zeroForOne true iff stock is currency0.
        bool zeroForOne = !_isIdrxCurrency0();

        uint256 feeBefore = _idrxClaimBalance();
        uint256 traderIdrxBefore = idrxToken.balanceOf(trader);

        vm.prank(trader);
        swapRouter.swap(
            poolKey,
            SwapParams({
                zeroForOne: zeroForOne,
                amountSpecified: -int256(amountIn),
                sqrtPriceLimitX96: zeroForOne ? TickMath.MIN_SQRT_PRICE + 1 : TickMath.MAX_SQRT_PRICE - 1
            }),
            PoolSwapTest.TestSettings({takeClaims: false, settleUsingBurn: false}),
            ""
        );

        uint256 feeTaken = _idrxClaimBalance() - feeBefore;
        uint256 received = idrxToken.balanceOf(trader) - traderIdrxBefore;

        assertGt(feeTaken, 0, "sell fee must be taken in IDRX");
        assertGt(received, 0, "trader must receive net IDRX");
        // fee is bps of gross output => fee / (received+fee) ~= 0.2%
        assertEq(feeTaken, ((received + feeTaken) * FEE_BPS) / 10_000, "sell fee must equal bps of gross IDRX output");
        assertEq(_stockClaimBalance(), 0, "hook must never hold stock-token fees");
    }

    // ─── Escape-plan mechanics (proven against real V4) ────────────────────

    function _buy(uint256 amountIn) internal {
        bool zeroForOne = _isIdrxCurrency0();
        vm.prank(trader);
        swapRouter.swap(
            poolKey,
            SwapParams({
                zeroForOne: zeroForOne,
                amountSpecified: -int256(amountIn),
                sqrtPriceLimitX96: zeroForOne ? TickMath.MIN_SQRT_PRICE + 1 : TickMath.MAX_SQRT_PRICE - 1
            }),
            PoolSwapTest.TestSettings({takeClaims: false, settleUsingBurn: false}),
            ""
        );
    }

    /// Circuit breaker: pausing the hook halts all swaps (attacker's only path).
    function test_pause_haltsSwaps() public {
        if (address(pm) == address(0)) return;
        hook.pause();
        bool zeroForOne = _isIdrxCurrency0();
        vm.prank(trader);
        vm.expectRevert(); // hook beforeSwap reverts (EnforcedPause), bubbled by PoolManager
        swapRouter.swap(
            poolKey,
            SwapParams({
                zeroForOne: zeroForOne,
                amountSpecified: -int256(uint256(100_000)),
                sqrtPriceLimitX96: zeroForOne ? TickMath.MIN_SQRT_PRICE + 1 : TickMath.MAX_SQRT_PRICE - 1
            }),
            PoolSwapTest.TestSettings({takeClaims: false, settleUsingBurn: false}),
            ""
        );
    }

    /// Escape hatch: even while paused, liquidity can still be removed —
    /// removeLiquidity is NOT hook-gated, so funds are never trapped.
    function test_pause_stillAllowsLiquidityRemoval() public {
        if (address(pm) == address(0)) return;
        hook.pause();

        int24 tickLower = (TickMath.MIN_TICK / TICK_SPACING) * TICK_SPACING;
        int24 tickUpper = (TickMath.MAX_TICK / TICK_SPACING) * TICK_SPACING;

        // Negative delta = remove. Must succeed despite the pause.
        liqRouter.modifyLiquidity(
            poolKey,
            ModifyLiquidityParams({tickLower: tickLower, tickUpper: tickUpper, liquidityDelta: -1e17, salt: bytes32(0)}),
            ""
        );
    }

    /// Fee rescue: accrued IDRX fees can be swept to feeRecipient during a pause.
    function test_handleHookFees_worksWhilePaused() public {
        if (address(pm) == address(0)) return;

        _buy(1_000_000); // accrue IDRX fee
        uint256 hookClaim = _idrxClaimBalance();
        assertGt(hookClaim, 0);

        hook.pause();

        uint256 recipientBefore = pm.balanceOf(hook.feeRecipient(), idrx.toId());

        Currency[] memory currencies = new Currency[](1);
        currencies[0] = idrx;
        hook.handleHookFees(currencies); // not pause-gated

        assertEq(_idrxClaimBalance(), 0, "hook fee claims must be swept out");
        assertEq(
            pm.balanceOf(hook.feeRecipient(), idrx.toId()) - recipientBefore,
            hookClaim,
            "feeRecipient must receive the full accrued IDRX fee even while paused"
        );
    }

    function test_zeroFee_takesNothing() public {
        if (address(pm) == address(0)) return;

        feeConfig.setSwapFeeBps(0);
        bool zeroForOne = _isIdrxCurrency0();

        vm.prank(trader);
        swapRouter.swap(
            poolKey,
            SwapParams({
                zeroForOne: zeroForOne,
                amountSpecified: -int256(uint256(500_000)),
                sqrtPriceLimitX96: zeroForOne ? TickMath.MIN_SQRT_PRICE + 1 : TickMath.MAX_SQRT_PRICE - 1
            }),
            PoolSwapTest.TestSettings({takeClaims: false, settleUsingBurn: false}),
            ""
        );

        assertEq(_idrxClaimBalance(), 0, "no fee when swapFeeBps is 0");
        assertEq(_stockClaimBalance(), 0, "no stock fee ever");
    }
}

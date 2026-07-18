// SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

import {Test} from "forge-std/Test.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import {PulsarProtocol} from "../../src/PulsarProtocol.sol";
import {PulsarStock} from "../../src/PulsarStock.sol";
import {PulsarSwapHook} from "../../src/v4/PulsarSwapHook.sol";
import {IDRX} from "../../src/mocks/IDRX.sol";
import {IUniswapV2Factory} from "../../src/interfaces/IUniswapV2Factory.sol";

import {IPoolManager} from "@uniswap/v4-core/src/interfaces/IPoolManager.sol";
import {Hooks} from "@uniswap/v4-core/src/libraries/Hooks.sol";
import {HookMiner} from "@uniswap/v4-periphery/src/utils/HookMiner.sol";

interface IV2Pair {
    function getReserves() external view returns (uint112, uint112, uint32);
    function token0() external view returns (address);
    function balanceOf(address) external view returns (uint256);
}

/// @notice End-to-end fork test of the full V4 migration path against the REAL
///         Uniswap V4 PoolManager on Arbitrum Sepolia: mint on V2 -> migrate the
///         liquidity to a V4 pool with the fee hook -> swap as a retail investor
///         (fee charged in IDRX by the hook) -> emergency withdraw (pause + pull
///         liquidity back). Runs only when RPC_URL is set.
contract PulsarProtocolV4ForkTest is Test {
    address constant POOL_MANAGER = 0xFB3e0C6F74eB1a21CC1Da29aeC80D2Dfe6C9a317;

    uint256 constant TOKEN_AMOUNT = 1_000 * 1e18;
    uint256 constant IDRX_AMOUNT = 2_500_000;
    int24 constant TICK_SPACING = 60;
    uint24 constant LP_FEE = 0;

    PulsarProtocol protocol;
    IDRX idrxToken;
    PulsarSwapHook hook;
    address uniswapFactory;
    address uniswapRouter;

    address admin = makeAddr("admin");
    address cust1 = makeAddr("cust1");
    address cust2 = makeAddr("cust2");
    address cust3 = makeAddr("cust3");
    address cust4 = makeAddr("cust4");
    address cust5 = makeAddr("cust5");
    address retail = makeAddr("retail");

    bool forked;

    function _deployFromArtifact(string memory path, bytes memory args) internal returns (address deployed) {
        bytes memory bytecode = vm.parseJsonBytes(vm.readFile(path), ".bytecode");
        bytes memory creationCode = abi.encodePacked(bytecode, args);
        assembly {
            deployed := create(0, add(creationCode, 0x20), mload(creationCode))
        }
        require(deployed != address(0) && deployed.code.length > 0, "artifact deploy failed");
    }

    function _sqrt(uint256 x) internal pure returns (uint256 y) {
        if (x == 0) return 0;
        uint256 z = (x + 1) / 2;
        y = x;
        while (z < y) {
            y = z;
            z = (x / z + z) / 2;
        }
    }

    function setUp() public {
        string memory rpc = vm.envOr("RPC_URL", string(""));
        if (bytes(rpc).length == 0) return;
        vm.createSelectFork(rpc);
        forked = true;

        uniswapFactory = _deployFromArtifact("script/artifacts/UniswapV2Factory.json", abi.encode(address(0)));
        uniswapRouter =
            _deployFromArtifact("script/artifacts/UniswapV2Router02.json", abi.encode(uniswapFactory, address(1)));

        vm.prank(admin);
        idrxToken = new IDRX(admin);

        address[] memory custodians = new address[](5);
        custodians[0] = cust1;
        custodians[1] = cust2;
        custodians[2] = cust3;
        custodians[3] = cust4;
        custodians[4] = cust5;

        PulsarProtocol impl = new PulsarProtocol();
        ERC1967Proxy proxy = new ERC1967Proxy(
            address(impl),
            abi.encodeCall(PulsarProtocol.initialize, (admin, uniswapRouter, address(idrxToken), custodians, admin))
        );
        protocol = PulsarProtocol(address(proxy));

        vm.startPrank(admin);
        idrxToken.mint(cust1, 100_000_000);
        idrxToken.mint(retail, 50_000_000);
        vm.stopPrank();

        // Mint BUMIP on V2 (deploys stock, creates V2 pool, protocol holds LP).
        vm.prank(cust1);
        uint256 pid =
            protocol.requestMint("BUMIP", "Pulsar Bumi Resources", "BUMI", TOKEN_AMOUNT, IDRX_AMOUNT, keccak256("a"));
        vm.prank(cust2);
        protocol.approveMint(pid);
        vm.prank(cust3);
        protocol.approveMint(pid);
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(pid);
        vm.stopPrank();

        // Deploy the hook at a mined address; feeConfig = protocol (reads swapFeeBps),
        // pauser = protocol (so emergencyWithdrawV4 can pause the hook).
        uint160 flags = uint160(
            Hooks.BEFORE_SWAP_FLAG | Hooks.AFTER_SWAP_FLAG | Hooks.BEFORE_SWAP_RETURNS_DELTA_FLAG
                | Hooks.AFTER_SWAP_RETURNS_DELTA_FLAG
        );
        bytes memory args =
            abi.encode(POOL_MANAGER, address(protocol), address(idrxToken), admin, address(protocol));
        (address hookAddr, bytes32 salt) =
            HookMiner.find(address(this), flags, type(PulsarSwapHook).creationCode, args);
        hook = new PulsarSwapHook{salt: salt}(
            IPoolManager(POOL_MANAGER), address(protocol), address(idrxToken), admin, address(protocol)
        );
        assertEq(address(hook), hookAddr);

        vm.startPrank(admin);
        protocol.configureV4(POOL_MANAGER, address(hook));
        protocol.setSwapFeeBps(20); // 0.2%
        vm.stopPrank();
    }

    function _currentV2SqrtPriceX96() internal view returns (uint160) {
        address stock = protocol.stocks("BUMIP");
        address pair = IUniswapV2Factory(uniswapFactory).getPair(stock, address(idrxToken));
        (uint112 r0, uint112 r1,) = IV2Pair(pair).getReserves();
        // order reserves to (currency0, currency1) as V4 sorts by address
        bool idrxIs0 = address(idrxToken) < stock;
        (uint256 res0, uint256 res1) = IV2Pair(pair).token0() == address(idrxToken)
            ? (uint256(r0), uint256(r1))
            : (uint256(r1), uint256(r0));
        // if v4 currency0 != v2 token0 ordering differs, but both sort by address identically
        idrxIs0; // silence
        uint256 ratioX192 = (res1 << 192) / res0;
        return uint160(_sqrt(ratioX192));
    }

    function test_migrate_swap_emergency_endToEnd() public {
        if (!forked) return;

        address stock = protocol.stocks("BUMIP");
        uint160 sqrtPriceX96 = _currentV2SqrtPriceX96();

        // ── Migrate V2 -> V4 (custodian) ──
        vm.prank(cust1);
        protocol.migrateV2ToV4("BUMIP", sqrtPriceX96, TICK_SPACING, LP_FEE, 0, 0);
        assertTrue(protocol.isV4Migrated("BUMIP"), "ticker must be marked migrated");

        // ── Retail buy on V4 (permissionless); hook charges 0.2% in IDRX ──
        uint256 amountIn = 1_000_000; // 10,000.00 IDRX
        uint256 stockBefore = PulsarStock(stock).balanceOf(retail);

        vm.startPrank(retail);
        idrxToken.approve(address(protocol), amountIn);
        protocol.swapV4("BUMIP", amountIn, 0, true);
        vm.stopPrank();

        assertGt(PulsarStock(stock).balanceOf(retail) - stockBefore, 0, "retail must receive pStock");
        // fee accrued to the hook as IDRX ERC-6909 claim (feeRecipient = protocol)
        uint256 feeClaim = IPoolManager(POOL_MANAGER).balanceOf(
            address(hook), uint256(uint160(address(idrxToken)))
        );
        assertEq(feeClaim, (amountIn * 20) / 10_000, "hook must hold exactly the 0.2% IDRX fee claim");

        // ── Emergency withdraw (custodian): pause hook + pull liquidity ──
        uint256 protoStockBefore = PulsarStock(stock).balanceOf(address(protocol));
        uint256 protoIdrxBefore = idrxToken.balanceOf(address(protocol));

        vm.prank(cust1);
        protocol.emergencyWithdrawV4("BUMIP");

        assertTrue(hook.paused(), "hook must be paused after emergency");
        assertFalse(protocol.isV4Migrated("BUMIP"), "migration flag cleared");
        assertGt(
            PulsarStock(stock).balanceOf(address(protocol)) - protoStockBefore, 0, "protocol recovers pStock"
        );
        assertGt(idrxToken.balanceOf(address(protocol)) - protoIdrxBefore, 0, "protocol recovers IDRX");

        // ── Swaps must be halted while paused ──
        vm.startPrank(retail);
        idrxToken.approve(address(protocol), 100_000);
        vm.expectRevert();
        protocol.swapV4("BUMIP", 100_000, 0, true);
        vm.stopPrank();
    }
}

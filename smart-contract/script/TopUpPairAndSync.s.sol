// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Script, console} from "forge-std/Script.sol";

interface IERC20BalanceTransfer {
    function balanceOf(address account) external view returns (uint256);
    function decimals() external view returns (uint8);
    function transfer(address to, uint256 value) external returns (bool);
}

interface IUniswapV2FactoryRead {
    function getPair(address tokenA, address tokenB) external view returns (address pair);
}

interface IUniswapV2PairSync {
    function token0() external view returns (address);
    function token1() external view returns (address);
    function getReserves() external view returns (uint112 reserve0, uint112 reserve1, uint32 blockTimestampLast);
    function sync() external;
}

contract TopUpPairAndSyncScript is Script {
    function run() external {
        uint256 custodianKey = vm.envUint("CUSTODIAN_1_PRIVATE_KEY");
        address factory = vm.envAddress("UNISWAP_V2_FACTORY");
        address idrx = vm.envAddress("IDRX");
        address stock = vm.envAddress("REBALANCE_STOCK");
        uint256 idxPrice = vm.envUint("REBALANCE_IDX_PRICE");
        uint256 lotSize = vm.envOr("REBALANCE_LOT_SIZE", uint256(100));

        address custodian = vm.addr(custodianKey);
        (address pair, uint256 idrxReserve, uint256 targetIdrxReserve, uint256 topUp) =
            _quoteTopUp(factory, idrx, stock, idxPrice, lotSize);
        uint256 balance = IERC20BalanceTransfer(idrx).balanceOf(custodian);
        require(balance >= topUp, "insufficient IDRX");

        console.log("Custodian:       ", custodian);
        console.log("Pair:            ", pair);
        console.log("Current IDRX raw:", idrxReserve);
        console.log("Target IDRX raw: ", targetIdrxReserve);
        console.log("Top-up IDRX raw: ", topUp);

        vm.startBroadcast(custodianKey);
        require(IERC20BalanceTransfer(idrx).transfer(pair, topUp), "IDRX transfer failed");
        IUniswapV2PairSync(pair).sync();
        vm.stopBroadcast();

        (uint112 newReserve0, uint112 newReserve1,) = IUniswapV2PairSync(pair).getReserves();
        address token0 = IUniswapV2PairSync(pair).token0();
        uint256 newIdrxReserve = token0 == idrx ? uint256(newReserve0) : uint256(newReserve1);
        console.log("New IDRX raw:    ", newIdrxReserve);
    }

    function _quoteTopUp(
        address factory,
        address idrx,
        address stock,
        uint256 idxPrice,
        uint256 lotSize
    ) internal view returns (address pair, uint256 idrxReserve, uint256 targetIdrxReserve, uint256 topUp) {
        pair = IUniswapV2FactoryRead(factory).getPair(idrx, stock);
        require(pair != address(0), "pair not found");

        (uint112 reserve0, uint112 reserve1,) = IUniswapV2PairSync(pair).getReserves();
        address token0 = IUniswapV2PairSync(pair).token0();
        idrxReserve = token0 == idrx ? uint256(reserve0) : uint256(reserve1);
        uint256 stockReserve = token0 == stock ? uint256(reserve0) : uint256(reserve1);

        targetIdrxReserve = stockReserve
            * idxPrice
            * lotSize
            * (10 ** IERC20BalanceTransfer(idrx).decimals())
            / (10 ** IERC20BalanceTransfer(stock).decimals());

        require(targetIdrxReserve > idrxReserve, "pair already at or above target");
        topUp = targetIdrxReserve - idrxReserve;
    }
}

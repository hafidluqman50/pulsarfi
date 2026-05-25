// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {console} from "forge-std/Script.sol";
import {OfficialUniswapV2Deployer} from "./OfficialUniswapV2.s.sol";
import {IDRX} from "../src/mocks/IDRX.sol";
import {PulsarProtocol} from "../src/PulsarProtocol.sol";
import {UUPSUpgradeable} from "@openzeppelin/contracts/proxy/utils/UUPSUpgradeable.sol";

contract UpgradeScript is OfficialUniswapV2Deployer {
    function run() external {
        uint256 deployerKey = vm.envUint("PRIVATE_KEY");
        address deployer = vm.addr(deployerKey);
        address proxy = vm.envAddress("PULSAR_PROTOCOL_PROXY");
        bool deployDex = vm.envOr("DEPLOY_UNISWAP_V2", false);
        bool deployMockIdrx = vm.envOr("DEPLOY_IDRX_MOCK", false);
        address newRouter = vm.envOr("NEW_ROUTER", address(0));
        address newIdrx = vm.envOr("NEW_IDRX", address(0));
        address mintIdrxTo = vm.envOr("MINT_IDRX_TO", address(0));
        uint256 mintIdrxAmount = vm.envOr("MINT_IDRX_AMOUNT", uint256(0));

        vm.startBroadcast(deployerKey);

        if (deployDex) {
            address weth = vm.envAddress("WETH");
            address factory = _deployOfficialUniswapV2Factory(deployer);
            newRouter = _deployOfficialUniswapV2Router(factory, weth);

            console.log("New UniswapV2Factory:", factory);
            console.log("New UniswapV2Router02:", newRouter);
        }

        if (deployMockIdrx) {
            IDRX idrxToken = new IDRX(deployer);
            newIdrx = address(idrxToken);

            if (mintIdrxTo != address(0) && mintIdrxAmount > 0) {
                idrxToken.mint(mintIdrxTo, mintIdrxAmount);
                console.log("Minted testnet IDRX to:", mintIdrxTo);
                console.log("Minted testnet IDRX:   ", mintIdrxAmount);
            }

            console.log("New testnet IDRX:", newIdrx);
        }

        PulsarProtocol newImplementation = new PulsarProtocol();
        UUPSUpgradeable(proxy).upgradeToAndCall(address(newImplementation), "");

        if (newRouter != address(0)) {
            PulsarProtocol(proxy).setRouter(newRouter);
            console.log("Router updated:      ", newRouter);
        }

        if (newIdrx != address(0)) {
            PulsarProtocol(proxy).setIDRX(newIdrx);
            console.log("IDRX updated:        ", newIdrx);
        }

        console.log("New implementation: ", address(newImplementation));
        console.log("Proxy (unchanged):  ", proxy);

        vm.stopBroadcast();
    }
}

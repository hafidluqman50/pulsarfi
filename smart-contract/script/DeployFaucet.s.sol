// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Script, console} from "forge-std/Script.sol";
import {IDRXFaucet} from "../src/mocks/IDRXFaucet.sol";
import {IDRX} from "../src/mocks/IDRX.sol";

contract DeployFaucetScript is Script {
    function run() external {
        uint256 deployerKey = vm.envUint("PRIVATE_KEY");
        address deployer    = vm.addr(deployerKey);
        address idrxAddr    = vm.envAddress("IDRX");

        // 500 Billion IDRX — 2 decimals → 50_000_000_000_000
        uint256 faucetFund  = 500_000_000_000 * 100;

        vm.startBroadcast(deployerKey);

        IDRXFaucet faucet = new IDRXFaucet(idrxAddr, deployer);
        console.log("IDRXFaucet deployed:", address(faucet));

        IDRX(idrxAddr).mint(address(faucet), faucetFund);
        console.log("Minted 500B IDRX to faucet");

        vm.stopBroadcast();
    }
}

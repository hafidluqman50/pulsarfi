// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Script, console} from "forge-std/Script.sol";
import {IDRX} from "../src/IDRX.sol";
import {PulsarProtocol} from "../src/PulsarProtocol.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";

contract DeployScript is Script {
    function run() external {
        uint256 deployerKey = vm.envUint("PRIVATE_KEY");
        address deployer    = vm.addr(deployerKey);
        address router      = vm.envAddress("UNISWAP_V2_ROUTER");
        address treasury    = vm.envAddress("TREASURY");

        address[] memory custodians = new address[](5);
        custodians[0] = vm.envAddress("CUSTODIAN_1");
        custodians[1] = vm.envAddress("CUSTODIAN_2");
        custodians[2] = vm.envAddress("CUSTODIAN_3");
        custodians[3] = vm.envAddress("CUSTODIAN_4");
        custodians[4] = vm.envAddress("CUSTODIAN_5");

        vm.startBroadcast(deployerKey);

        // Deploy mock IDRX — deployer is temporary owner, transferred to protocol below
        IDRX idrxToken = new IDRX(deployer);

        PulsarProtocol implementation = new PulsarProtocol();

        bytes memory initData = abi.encodeCall(
            PulsarProtocol.initialize,
            (deployer, router, address(idrxToken), custodians, treasury)
        );

        ERC1967Proxy proxy = new ERC1967Proxy(address(implementation), initData);

        // Transfer IDRX mint rights to PulsarProtocol so _provideToPool can mint
        idrxToken.transferOwnership(address(proxy));

        console.log("IDRX mock:                    ", address(idrxToken));
        console.log("PulsarProtocol implementation:", address(implementation));
        console.log("PulsarProtocol proxy:         ", address(proxy));

        vm.stopBroadcast();
    }
}

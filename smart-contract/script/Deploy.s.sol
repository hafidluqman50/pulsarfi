// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {console} from "forge-std/Script.sol";
import {OfficialUniswapV2Deployer} from "./OfficialUniswapV2.s.sol";
import {IDRX} from "../src/mocks/IDRX.sol";
import {PulsarProtocol} from "../src/PulsarProtocol.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";

contract DeployScript is OfficialUniswapV2Deployer {
    function run() external {
        uint256 deployerKey = vm.envUint("PRIVATE_KEY");
        address deployer = vm.addr(deployerKey);
        address treasury = vm.envAddress("TREASURY");
        address weth = vm.envAddress("WETH");

        address[] memory custodians = new address[](5);
        custodians[0] = vm.envAddress("CUSTODIAN_1");
        custodians[1] = vm.envAddress("CUSTODIAN_2");
        custodians[2] = vm.envAddress("CUSTODIAN_3");
        custodians[3] = vm.envAddress("CUSTODIAN_4");
        custodians[4] = vm.envAddress("CUSTODIAN_5");

        vm.startBroadcast(deployerKey);

        // 1. Deploy official Uniswap V2 bytecode artifacts. Router pairFor hash must match factory pair bytecode.
        address factory = _deployOfficialUniswapV2Factory(deployer);
        address router = _deployOfficialUniswapV2Router(factory, weth);

        // 2. Deploy testnet IDRX. On mainnet, configure the protocol with the real IDRX address.
        IDRX idrxToken = new IDRX(deployer);

        // 3. Deploy PulsarProtocol (UUPS proxy)
        PulsarProtocol implementation = new PulsarProtocol();

        bytes memory initData =
            abi.encodeCall(PulsarProtocol.initialize, (deployer, router, address(idrxToken), custodians, treasury));

        ERC1967Proxy proxy = new ERC1967Proxy(address(implementation), initData);

        console.log("UniswapV2Factory:              ", factory);
        console.log("UniswapV2Router02:             ", router);
        console.log("IDRX:                          ", address(idrxToken));
        console.log("PulsarProtocol implementation: ", address(implementation));
        console.log("PulsarProtocol proxy:          ", address(proxy));

        vm.stopBroadcast();
    }
}

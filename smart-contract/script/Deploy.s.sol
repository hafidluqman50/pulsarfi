// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Script, console} from "forge-std/Script.sol";
import {PStockFactory} from "../src/PStockFactory.sol";

contract DeployScript is Script {
    function run() external {
        uint256 deployerKey = vm.envUint("PRIVATE_KEY");
        address deployer    = vm.addr(deployerKey);

        vm.startBroadcast(deployerKey);

        PStockFactory factory = new PStockFactory(deployer);
        console.log("PStockFactory deployed at:", address(factory));

        // Deploy all 8 pStocks
        address pBUMI = factory.deploy("Pulsar Bumi Resources",         "pBUMI", "BUMI");
        address pENRG = factory.deploy("Pulsar Energi Mega",             "pENRG", "ENRG");
        address pKIJA = factory.deploy("Pulsar Kawasan Industri",        "pKIJA", "KIJA");
        address pTLKM = factory.deploy("Pulsar Telkom Indonesia",        "pTLKM", "TLKM");
        address pBBRI = factory.deploy("Pulsar Bank Rakyat",             "pBBRI", "BBRI");
        address pGOTO = factory.deploy("Pulsar GoTo Gojek Tokopedia",    "pGOTO", "GOTO");
        address pASII = factory.deploy("Pulsar Astra International",     "pASII", "ASII");
        address pUNVR = factory.deploy("Pulsar Unilever Indonesia",      "pUNVR", "UNVR");

        console.log("pBUMI:", pBUMI);
        console.log("pENRG:", pENRG);
        console.log("pKIJA:", pKIJA);
        console.log("pTLKM:", pTLKM);
        console.log("pBBRI:", pBBRI);
        console.log("pGOTO:", pGOTO);
        console.log("pASII:", pASII);
        console.log("pUNVR:", pUNVR);

        vm.stopBroadcast();
    }
}

// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Script, console} from "forge-std/Script.sol";
import {AgentTaskManager} from "../src/AgentTaskManager.sol";

/// @notice Deploys AgentTaskManager against the already-live PulsarProtocol
/// proxy and grants AGENT_ROLE to the dedicated AGENT_WALLET — a separate
/// wallet from the deployer, already provisioned in .env, that the backend
/// signs with for createTask/grantTradePermission/recordSubTasks/
/// executeTrade/cancelTask (see backend/src/onchain/agenttaskmanager).
contract DeployAgentTaskManagerScript is Script {
    function run() external {
        uint256 deployerKey = vm.envUint("PRIVATE_KEY");
        address deployer = vm.addr(deployerKey);
        address protocol = vm.envAddress("PULSAR_PROTOCOL_PROXY");
        address agentWallet = vm.envAddress("AGENT_WALLET");

        vm.startBroadcast(deployerKey);

        AgentTaskManager manager = new AgentTaskManager(protocol, deployer);
        manager.grantRole(manager.AGENT_ROLE(), agentWallet);

        vm.stopBroadcast();

        console.log("AgentTaskManager deployed at:", address(manager));
        console.log("AGENT_ROLE granted to:", agentWallet);
    }
}

// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Script, console} from "forge-std/Script.sol";
import {PulsarProtocol} from "../src/PulsarProtocol.sol";
import {IDRX} from "../src/mocks/IDRX.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";

contract Mint200kTokensLiveScript is Script {
    struct MintConfig {
        string ticker;
        string stockName;
        string idxTicker;
        uint256 priceIdrx; // in IDRX whole units (e.g. 525000 for PTROP)
    }

    function run() external {
        uint256 deployerKey   = vm.envUint("PRIVATE_KEY");
        uint256 custodian1Key = vm.envUint("CUSTODIAN_1_PRIVATE_KEY");
        uint256 custodian2Key = vm.envUint("CUSTODIAN_2_PRIVATE_KEY");
        uint256 custodian3Key = vm.envUint("CUSTODIAN_3_PRIVATE_KEY");

        address custodian1 = vm.addr(custodian1Key);
        address idrxAddr   = vm.envAddress("IDRX");
        address proxy      = vm.envAddress("PULSAR_PROTOCOL_PROXY");

        PulsarProtocol protocol = PulsarProtocol(proxy);

        // Configure tickers to mint 200,000 tokens (1 token = 1 lot)
        MintConfig[] memory configs = new MintConfig[](6);
        configs[0] = MintConfig("PTROP", "Pulsar Petrosea", "PTRO", 525_000);
        configs[1] = MintConfig("BMRIP", "Pulsar Bank Mandiri", "BMRI", 417_000);
        configs[2] = MintConfig("BRPTP", "Pulsar Barito Pacific", "BRPT", 187_000);
        configs[3] = MintConfig("BBCAP", "Pulsar Bank Central Asia", "BBCA", 637_500);
        configs[4] = MintConfig("BBRIP", "Pulsar Bank Rakyat", "BBRI", 308_000);
        configs[5] = MintConfig("BDMNP", "Pulsar Bank Danamon", "BDMN", 429_000);

        uint256 tokenAmount = 200_000 * 1e18;

        for (uint256 i = 0; i < configs.length; i++) {
            MintConfig memory cfg = configs[i];
            console.log("-----------------------------------------");
            console.log("Processing 200k mint for ticker:", cfg.ticker);

            if (protocol.hasPendingRequest(cfg.ticker)) {
                console.log("Ticker already has a pending mint request. Skipping request step.");
                continue;
            }

            uint256 idrxAmount = 200_000 * cfg.priceIdrx * 100; // 2 decimals for IDRX

            // 1. Mint IDRX to Custodian 1
            console.log("Step 1: Minting IDRX to Custodian 1...");
            vm.startBroadcast(deployerKey);
            IDRX(idrxAddr).mint(custodian1, idrxAmount);
            vm.stopBroadcast();

            // 2. Custodian 1 approves IDRX and requests mint
            console.log("Step 2: Custodian 1 approving IDRX and requesting mint...");
            bytes32 attestationHash = keccak256(abi.encodePacked(cfg.ticker, tokenAmount, idrxAmount, block.timestamp));
            vm.startBroadcast(custodian1Key);
            IERC20(idrxAddr).approve(proxy, idrxAmount);
            uint256 proposalId = protocol.requestMint(
                cfg.ticker,
                cfg.stockName,
                cfg.idxTicker,
                tokenAmount,
                idrxAmount,
                attestationHash
            );
            vm.stopBroadcast();
            console.log("Mint proposal created on-chain. Proposal ID:", proposalId);

            // 3. Custodian 2 approves mint
            console.log("Step 3: Custodian 2 approving proposal...");
            vm.startBroadcast(custodian2Key);
            protocol.approveMint(proposalId);
            vm.stopBroadcast();

            // 4. Custodian 3 approves mint (reaches 3/5 threshold)
            console.log("Step 4: Custodian 3 approving proposal...");
            vm.startBroadcast(custodian3Key);
            protocol.approveMint(proposalId);
            vm.stopBroadcast();

            // 5. Custodian 1 executes mint (provides full-range V4 liquidity)
            console.log("Step 5: Custodian 1 executing mint into Uniswap V4 pool...");
            vm.startBroadcast(custodian1Key);
            protocol.executeMint(proposalId, 0);
            vm.stopBroadcast();

            address stockAddr = protocol.stocks(cfg.ticker);
            uint256 supply = IERC20(stockAddr).totalSupply();
            console.log("SUCCESS! New total supply for", cfg.ticker);
            console.log("Total Supply (wei):", supply);
        }
    }
}

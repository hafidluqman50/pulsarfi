// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Test, console} from "forge-std/Test.sol";
import {PulsarProtocol} from "../src/PulsarProtocol.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";

contract Mint200kForkTest is Test {
    address constant PROXY = 0x204488318C0E75978B3c851382Aa83f3065a8f5A;
    address constant ADMIN = 0x5617007a51331f4e7241DE852cDc4Bbbe723CAE4;

    address constant CUSTODIAN_1 = 0xC7d7D4DeCc87cE8713dD1a49bF81C65aD6c20419;
    address constant CUSTODIAN_2 = 0x332dAE41D2a608E1D2853A80dfd12D4ed710323E;
    address constant CUSTODIAN_3 = 0x462a9493f5e4d3118f634c97988B51B3630998f4;

    PulsarProtocol protocol;
    address idrxAddr;

    function setUp() public {
        string memory rpc = vm.envOr("RPC_URL", string(""));
        if (bytes(rpc).length == 0) return;
        vm.createSelectFork(rpc);

        protocol = PulsarProtocol(PROXY);
        idrxAddr = protocol.idrx();
    }

    function test_mint200kPTROP_and_swap10MillionIDRX() public {
        // PTROP price is 525,000 IDRX. 1 token = 1 lot.
        // For 200,000 tokens: tokenAmount = 200,000 * 1e18
        // idrxAmount = 200,000 * 525,000 IDRX = 105,000,000,000 IDRX = 10,500,000,000,000 raw IDRX (2 decimals)
        uint256 tokenAmount = 200_000 * 1e18;
        uint256 idrxAmount = 200_000 * 525_000 * 100; // 10.5 Trillion raw units

        // Mint IDRX to CUSTODIAN_1 from ADMIN
        vm.prank(ADMIN);
        (bool ok,) = idrxAddr.call(abi.encodeWithSignature("mint(address,uint256)", CUSTODIAN_1, idrxAmount));
        assertTrue(ok, "mint IDRX failed");

        // CUSTODIAN_1 approves IDRX to PROXY
        vm.prank(CUSTODIAN_1);
        IERC20(idrxAddr).approve(PROXY, idrxAmount);

        // Step 1: Request Mint
        bytes32 attestationHash = keccak256(abi.encodePacked("PTROP", tokenAmount, idrxAmount, block.timestamp));
        vm.prank(CUSTODIAN_1);
        uint256 proposalId = protocol.requestMint(
            "PTROP",
            "Petrosea Tbk",
            "PTRO",
            tokenAmount,
            idrxAmount,
            attestationHash
        );
        console.log("Proposal ID created:", proposalId);

        // Step 2: Approve Mint by Custodian 2 and Custodian 3
        vm.prank(CUSTODIAN_2);
        protocol.approveMint(proposalId);

        vm.prank(CUSTODIAN_3);
        protocol.approveMint(proposalId);

        // Step 3: Execute Mint by Custodian 1
        vm.prank(CUSTODIAN_1);
        protocol.executeMint(proposalId, 0);
        console.log("Execute mint succeeded!");

        // Verify new PTROP totalSupply
        address ptropAddr = protocol.stocks("PTROP");
        uint256 supply = IERC20(ptropAddr).totalSupply();
        console.log("New PTROP Total Supply:", supply);
        assertGe(supply, 200_000 * 1e18, "Supply should be >= 200k");

        // Now test 10 Million IDRX swap with 1% slippage
        address buyer = makeAddr("buyer");
        uint256 idrxSwapIn = 10_000_000 * 100; // 10 Million IDRX = 1,000,000,000 raw units

        vm.prank(ADMIN);
        (bool okBuyer,) = idrxAddr.call(abi.encodeWithSignature("mint(address,uint256)", buyer, idrxSwapIn));
        assertTrue(okBuyer, "mint IDRX to buyer failed");

        vm.prank(buyer);
        IERC20(idrxAddr).approve(PROXY, idrxSwapIn);

        // Linear expectation: 10M / 525k = ~19.04 PTROP. 1% slippage floor = ~18.85 PTROP
        uint256 minOut = 18_850_000_000_000_000_000; // 18.85 PTROP (1% slippage)
        vm.prank(buyer);
        protocol.swapV4("PTROP", idrxSwapIn, minOut, true);

        uint256 buyerBalance = IERC20(ptropAddr).balanceOf(buyer);
        console.log("Buyer received PTROP tokens:", buyerBalance);
        assertGe(buyerBalance, minOut, "Buyer should receive >= minOut with deep liquidity!");
    }
}

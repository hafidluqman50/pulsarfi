// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Test} from "forge-std/Test.sol";
import {PulsarStock} from "../src/PulsarStock.sol";

contract PulsarStockTest is Test {
    PulsarStock stock;
    address admin = address(this);
    address user  = address(0xBEEF);

    function setUp() public {
        stock = new PulsarStock("Pulsar Bumi Resources", "BUMIP", "BUMI", admin);
    }

    function test_mint() public {
        bytes32 attestHash = keccak256("test-attestation-1");
        stock.mint(user, 1_000e18, attestHash);
        assertEq(stock.balanceOf(user), 1_000e18);
        assertEq(stock.totalSupply(), 1_000e18);
    }

    function test_burn() public {
        stock.mint(user, 500e18, keccak256("mint-1"));
        stock.burn(user, 200e18, keccak256("burn-1"));
        assertEq(stock.balanceOf(user), 300e18);
    }

    function test_onlyMinterCanMint() public {
        vm.prank(user);
        vm.expectRevert();
        stock.mint(user, 1e18, bytes32(0));
    }

    function test_decimals() public view {
        assertEq(stock.decimals(), 18);
    }

    function test_idxTicker() public view {
        assertEq(stock.idxTicker(), "BUMI");
    }
}

// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Test, console} from "forge-std/Test.sol";
import {PStock} from "../src/PStock.sol";
import {PStockFactory} from "../src/PStockFactory.sol";

contract PStockTest is Test {
    PStockFactory factory;
    PStock        pBUMI;
    address       admin   = address(this);
    address       user    = address(0xBEEF);

    function setUp() public {
        factory = new PStockFactory(admin);
        address addr = factory.deploy("Pulsar Bumi Resources", "pBUMI", "BUMI");
        pBUMI = PStock(addr);
    }

    function test_mint() public {
        bytes32 attestHash = keccak256("test-attestation-1");
        pBUMI.mint(user, 1_000e18, attestHash);
        assertEq(pBUMI.balanceOf(user), 1_000e18);
        assertEq(pBUMI.totalSupply(), 1_000e18);
    }

    function test_burn() public {
        bytes32 mintHash = keccak256("mint-1");
        bytes32 burnHash = keccak256("burn-1");
        pBUMI.mint(user, 500e18, mintHash);
        pBUMI.burn(user, 200e18, burnHash);
        assertEq(pBUMI.balanceOf(user), 300e18);
    }

    function test_onlyMinterCanMint() public {
        vm.prank(user);
        vm.expectRevert();
        pBUMI.mint(user, 1e18, bytes32(0));
    }

    function test_factory_getAll() public {
        factory.deploy("Pulsar Energi Mega", "pENRG", "ENRG");
        (string[] memory tickers, address[] memory addrs) = factory.getAll();
        assertEq(tickers.length, 2);
        assertEq(tickers[0], "pBUMI");
        assertEq(tickers[1], "pENRG");
        assertTrue(addrs[0] != address(0));
        assertTrue(addrs[1] != address(0));
    }

    function test_decimals() public view {
        assertEq(pBUMI.decimals(), 18);
    }

    function test_idxTicker() public view {
        assertEq(pBUMI.idxTicker(), "BUMI");
    }
}

// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

/// @notice Mock IDRX stablecoin for Arbitrum Sepolia testnet.
///         On mainnet, use the real IDRX contract address instead.
contract IDRX is ERC20, Ownable {
    constructor(address owner) ERC20("IDR Stablecoin", "IDRX") Ownable(owner) {}

    function mint(address to, uint256 amount) external onlyOwner {
        _mint(to, amount);
    }

    function decimals() public pure override returns (uint8) {
        return 2;
    }
}

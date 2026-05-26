// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

/// @notice 1:1 tokenized IDX equity on Arbitrum Sepolia.
///         Exclusively controlled by PulsarProtocol (owner).
contract PulsarStock is ERC20, Ownable {
    string public idxTicker;

    event Minted(address indexed to, uint256 amount, bytes32 attestationHash);
    event Burned(address indexed from, uint256 amount, bytes32 attestationHash);

    constructor(
        string memory name_,
        string memory symbol_,
        string memory idxTicker_,
        address owner_
    ) ERC20(name_, symbol_) Ownable(owner_) {
        idxTicker = idxTicker_;
    }

    function mint(address to, uint256 amount, bytes32 attestationHash) external onlyOwner {
        _mint(to, amount);
        emit Minted(to, amount, attestationHash);
    }

    function burn(address from, uint256 amount, bytes32 attestationHash) external onlyOwner {
        _burn(from, amount);
        emit Burned(from, amount, attestationHash);
    }

    function decimals() public pure override returns (uint8) {
        return 18;
    }
}

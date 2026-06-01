// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

/// @notice Public testnet faucet for mock IDRX on Arbitrum Sepolia.
///         Anyone can call drip() once per 24 hours to receive DRIP_AMOUNT IDRX.
contract IDRXFaucet is Ownable {
    IERC20 public immutable idrx;

    uint256 public constant DRIP_AMOUNT = 10_000_000; // 100,000 IDRX (2 decimals)
    uint256 public constant COOLDOWN    = 1 days;

    mapping(address => uint256) public lastDrip;

    event Dripped(address indexed to, uint256 amount);
    event Withdrawn(address indexed to, uint256 amount);

    constructor(address idrx_, address owner_) Ownable(owner_) {
        idrx = IERC20(idrx_);
    }

    function drip() external {
        require(block.timestamp >= lastDrip[msg.sender] + COOLDOWN, "Cooldown: wait 24h");
        require(idrx.balanceOf(address(this)) >= DRIP_AMOUNT, "Faucet empty");

        lastDrip[msg.sender] = block.timestamp;
        idrx.transfer(msg.sender, DRIP_AMOUNT);

        emit Dripped(msg.sender, DRIP_AMOUNT);
    }

    function withdraw(address to, uint256 amount) external onlyOwner {
        idrx.transfer(to, amount);
        emit Withdrawn(to, amount);
    }
}

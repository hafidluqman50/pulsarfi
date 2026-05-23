// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

/// @notice 1:1 tokenized IDX equity on Arbitrum Sepolia.
///         Only MINTER_ROLE can mint (custodian bridge) and BURNER_ROLE can burn.
contract PStock is ERC20, AccessControl {
    bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");
    bytes32 public constant BURNER_ROLE = keccak256("BURNER_ROLE");

    string public idxTicker;

    event Minted(address indexed to,     uint256 amount, bytes32 attestationHash);
    event Burned(address indexed from,   uint256 amount, bytes32 attestationHash);

    constructor(
        string memory name_,
        string memory symbol_,
        string memory idxTicker_,
        address admin
    ) ERC20(name_, symbol_) {
        idxTicker = idxTicker_;
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
        _grantRole(MINTER_ROLE, admin);
        _grantRole(BURNER_ROLE, admin);
    }

    /// @notice Mint pStock tokens 1:1 against custodian attestation.
    function mint(address to, uint256 amount, bytes32 attestationHash) external onlyRole(MINTER_ROLE) {
        _mint(to, amount);
        emit Minted(to, amount, attestationHash);
    }

    /// @notice Burn pStock tokens when underlying IDX equity is redeemed.
    function burn(address from, uint256 amount, bytes32 attestationHash) external onlyRole(BURNER_ROLE) {
        _burn(from, amount);
        emit Burned(from, amount, attestationHash);
    }

    function decimals() public pure override returns (uint8) {
        return 18;
    }
}

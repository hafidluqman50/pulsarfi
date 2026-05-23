// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {PStock} from "./PStock.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";

/// @notice Deploys and tracks all pStock ERC-20 contracts.
contract PStockFactory is AccessControl {
    bytes32 public constant OPERATOR_ROLE = keccak256("OPERATOR_ROLE");

    mapping(string => address) public pStocks;
    string[] public tickers;

    event PStockDeployed(string indexed ticker, string idxTicker, address contractAddress);

    constructor(address admin) {
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
        _grantRole(OPERATOR_ROLE, admin);
    }

    function deploy(
        string calldata name,
        string calldata symbol,
        string calldata idxTicker
    ) external onlyRole(OPERATOR_ROLE) returns (address) {
        require(pStocks[symbol] == address(0), "already deployed");
        PStock token = new PStock(name, symbol, idxTicker, msg.sender);
        pStocks[symbol] = address(token);
        tickers.push(symbol);
        emit PStockDeployed(symbol, idxTicker, address(token));
        return address(token);
    }

    function getAll() external view returns (string[] memory, address[] memory) {
        address[] memory addrs = new address[](tickers.length);
        for (uint256 i = 0; i < tickers.length; i++) {
            addrs[i] = pStocks[tickers[i]];
        }
        return (tickers, addrs);
    }
}

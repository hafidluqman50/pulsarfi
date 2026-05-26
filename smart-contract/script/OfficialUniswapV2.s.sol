// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Script} from "forge-std/Script.sol";

abstract contract OfficialUniswapV2Deployer is Script {
    string private constant FACTORY_ARTIFACT = "script/artifacts/UniswapV2Factory.json";
    string private constant ROUTER_ARTIFACT = "script/artifacts/UniswapV2Router02.json";

    function _deployOfficialUniswapV2Factory(address feeToSetter) internal returns (address factory) {
        factory = _deployFromArtifact(FACTORY_ARTIFACT, abi.encode(feeToSetter));
    }

    function _deployOfficialUniswapV2Router(address factory, address weth) internal returns (address router) {
        router = _deployFromArtifact(ROUTER_ARTIFACT, abi.encode(factory, weth));
    }

    function _deployFromArtifact(string memory artifactPath, bytes memory constructorArgs)
        private
        returns (address deployed)
    {
        bytes memory bytecode = vm.parseJsonBytes(vm.readFile(artifactPath), ".bytecode");
        bytes memory creationCode = abi.encodePacked(bytecode, constructorArgs);

        assembly {
            deployed := create(0, add(creationCode, 0x20), mload(creationCode))
        }

        require(deployed != address(0) && deployed.code.length > 0, "artifact deploy failed");
    }
}

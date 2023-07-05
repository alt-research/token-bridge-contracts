// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.4;

import { L2GatewayRouter } from "./gateway/L2GatewayRouter.sol";
import { L2ERC20Gateway } from "./gateway/L2ERC20Gateway.sol";
import { L2CustomGateway } from "./gateway/L2CustomGateway.sol";
import { StandardArbERC20 } from "./StandardArbERC20.sol";
import { BeaconProxyFactory } from "../libraries/ClonableBeaconProxy.sol";
import { UpgradeableBeacon } from "@openzeppelin/contracts/proxy/beacon/UpgradeableBeacon.sol";
import { ProxyAdmin } from "@openzeppelin/contracts/proxy/transparent/ProxyAdmin.sol";
import { TransparentUpgradeableProxy } from "@openzeppelin/contracts/proxy/transparent/TransparentUpgradeableProxy.sol";
import { Create2 } from "@openzeppelin/contracts/utils/Create2.sol";

contract L2AtomicTokenBridgeFactory {
    function deployL2Contracts(
        bytes memory routerCreationCode,
        bytes memory standardGatewayCreationCode,
        bytes memory customGatewayCreationCode,
        address l1Router,
        address l1StandardGateway,
        address l1CustomGateway,
        address l2StandardGatewayExpectedAddress,
        address proxyAdminOwner
    ) external {
        // create proxyAdmin which will be used for all contracts
        address proxyAdmin = address(new ProxyAdmin{ salt: _getSaltFrom(L2Salts.PROXY_ADMIN) }());

        address router = _deployRouter(
            routerCreationCode,
            l1Router,
            l2StandardGatewayExpectedAddress,
            proxyAdmin
        );
        _deployStandardGateway(standardGatewayCreationCode, l1StandardGateway, router, proxyAdmin);
        _deployCustomGateway(customGatewayCreationCode, l1CustomGateway, router, proxyAdmin);

        // transfer ownership to L1Creator's msg.sender
        ProxyAdmin(proxyAdmin).transferOwnership(proxyAdminOwner);
    }

    function _deployRouter(
        bytes memory creationCode,
        address l1Router,
        address l2StandardGatewayExpectedAddress,
        address proxyAdmin
    ) internal returns (address) {
        // create logic and proxy
        address routerLogicAddress = Create2.deploy(
            0,
            _getSaltFrom(L2Salts.ROUTER_LOGIC),
            creationCode
        );
        L2GatewayRouter router = L2GatewayRouter(
            address(
                new TransparentUpgradeableProxy{ salt: _getSaltFrom(L2Salts.ROUTER) }(
                    routerLogicAddress,
                    proxyAdmin,
                    bytes("")
                )
            )
        );

        // init
        router.initialize(l1Router, l2StandardGatewayExpectedAddress);

        return address(router);
    }

    function _deployStandardGateway(
        bytes memory creationCode,
        address l1StandardGateway,
        address router,
        address proxyAdmin
    ) internal {
        // create logic and proxy
        address standardGatewayLogicAddress = Create2.deploy(
            0,
            _getSaltFrom(L2Salts.STANDARD_GATEWAY_LOGIC),
            creationCode
        );
        L2ERC20Gateway standardGateway = L2ERC20Gateway(
            address(
                new TransparentUpgradeableProxy{ salt: _getSaltFrom(L2Salts.STANDARD_GATEWAY) }(
                    standardGatewayLogicAddress,
                    proxyAdmin,
                    bytes("")
                )
            )
        );

        // create beacon
        StandardArbERC20 standardArbERC20 = new StandardArbERC20{
            salt: _getSaltFrom(L2Salts.STANDARD_ERC20)
        }();
        UpgradeableBeacon beacon = new UpgradeableBeacon{
            salt: _getSaltFrom(L2Salts.UPGRADEABLE_BEACON)
        }(address(standardArbERC20));
        BeaconProxyFactory beaconProxyFactory = new BeaconProxyFactory{
            salt: _getSaltFrom(L2Salts.BEACON_PROXY_FACTORY)
        }();

        // init contracts
        beaconProxyFactory.initialize(address(beacon));
        standardGateway.initialize(l1StandardGateway, router, address(beaconProxyFactory));
    }

    function _deployCustomGateway(
        bytes memory creationCode,
        address l1CustomGateway,
        address router,
        address proxyAdmin
    ) internal {
        address customGatewayLogicAddress = Create2.deploy(
            0,
            _getSaltFrom(L2Salts.CUSTOM_GATEWAY_LOGIC),
            creationCode
        );

        // create logic and proxy
        L2CustomGateway customGateway = L2CustomGateway(
            address(
                new TransparentUpgradeableProxy{ salt: _getSaltFrom(L2Salts.CUSTOM_GATEWAY) }(
                    customGatewayLogicAddress,
                    proxyAdmin,
                    bytes("")
                )
            )
        );

        // init
        customGateway.initialize(l1CustomGateway, router);
    }

    function _getSaltFrom(bytes32 prefix) internal view returns (bytes32) {
        return keccak256(abi.encodePacked(prefix, msg.sender));
    }
}

/**
 * Collection of salts used in CREATE2 deployment of L2 token bridge contracts.
 */
library L2Salts {
    bytes32 public constant PROXY_ADMIN = keccak256(bytes("OrbitL2ProxyAdmin"));
    bytes32 public constant ROUTER_LOGIC = keccak256(bytes("OrbitL2GatewayRouterLogic"));
    bytes32 public constant ROUTER = keccak256(bytes("OrbitL2GatewayRouterProxy"));
    bytes32 public constant STANDARD_GATEWAY_LOGIC =
        keccak256(bytes("OrbitL2StandardGatewayLogic"));
    bytes32 public constant STANDARD_GATEWAY = keccak256(bytes("OrbitL2StandardGatewayProxy"));
    bytes32 public constant CUSTOM_GATEWAY_LOGIC = keccak256(bytes("OrbitL2CustomGatewayLogic"));
    bytes32 public constant CUSTOM_GATEWAY = keccak256(bytes("OrbitL2CustomGatewayProxy"));
    bytes32 public constant STANDARD_ERC20 = keccak256(bytes("OrbitStandardArbERC20"));
    bytes32 public constant UPGRADEABLE_BEACON = keccak256(bytes("OrbitUpgradeableBeacon"));
    bytes32 public constant BEACON_PROXY_FACTORY = keccak256(bytes("OrbitBeaconProxyFactory"));
}
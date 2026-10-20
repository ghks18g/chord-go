// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title ChordForwarder
 * @notice EIP-712 & EIP-2771 compliant Meta-Transaction Forwarder for CHORD L1
 * @dev Verifies off-chain EIP-712 signatures and forwards calls with the sender appended to calldata.
 */
contract ChordForwarder {
    struct ForwardRequest {
        address from;
        address to;
        uint256 value;
        uint256 gas;
        uint256 nonce;
        uint256 deadline;
        bytes data;
    }

    bytes32 public constant TYPEHASH =
        keccak256(
            "ForwardRequest(address from,address to,uint256 value,uint256 gas,uint256 nonce,uint256 deadline,bytes data)"
        );

    bytes32 private immutable _DOMAIN_SEPARATOR;
    mapping(address => uint256) private _nonces;

    event Executed(address indexed from, address indexed to, bool success, bytes returndata);

    constructor() {
        _DOMAIN_SEPARATOR = keccak256(
            abi.encode(
                keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"),
                keccak256(bytes("ChordForwarder")),
                keccak256(bytes("1")),
                block.chainid,
                address(this)
            )
        );
    }

    function domainSeparator() external view returns (bytes32) {
        return _DOMAIN_SEPARATOR;
    }

    function getNonce(address from) external view returns (uint256) {
        return _nonces[from];
    }

    function verify(ForwardRequest calldata req, bytes calldata signature) public view returns (bool) {
        if (block.timestamp > req.deadline) {
            return false;
        }
        if (_nonces[req.from] != req.nonce) {
            return false;
        }

        bytes32 structHash = keccak256(
            abi.encode(
                TYPEHASH,
                req.from,
                req.to,
                req.value,
                req.gas,
                req.nonce,
                req.deadline,
                keccak256(req.data)
            )
        );

        bytes32 digest = keccak256(
            abi.encodePacked("\x19\x01", _DOMAIN_SEPARATOR, structHash)
        );

        address signer = _recover(digest, signature);
        return signer == req.from && signer != address(0);
    }

    function execute(ForwardRequest calldata req, bytes calldata signature)
        external
        payable
        returns (bool, bytes memory)
    {
        require(verify(req, signature), "ChordForwarder: signature or deadline verification failed");

        _nonces[req.from]++;

        // EIP-2771: append raw sender (20 bytes) to the calldata
        bytes memory callDataWithSender = abi.encodePacked(req.data, req.from);

        (bool success, bytes memory returndata) = req.to.call{gas: req.gas, value: req.value}(callDataWithSender);

        emit Executed(req.from, req.to, success, returndata);

        return (success, returndata);
    }

    function _recover(bytes32 digest, bytes memory signature) internal pure returns (address) {
        if (signature.length != 65) {
            return address(0);
        }

        bytes32 r;
        bytes32 s;
        uint8 v;

        assembly {
            r := mload(add(signature, 0x20))
            s := mload(add(signature, 0x40))
            v := byte(0, mload(add(signature, 0x60)))
        }

        if (v < 27) {
            v += 27;
        }

        if (v != 27 && v != 28) {
            return address(0);
        }

        return ecrecover(digest, v, r, s);
    }
}

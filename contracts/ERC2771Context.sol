// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @dev Context variant with ERC-2771 support for native meta transactions.
 * Extracted from OpenZeppelin Contracts (v5.0.0).
 */
abstract contract ERC2771Context {
    address private immutable _trustedForwarder;

    constructor(address trustedForwarder_) {
        _trustedForwarder = trustedForwarder_;
    }

    /**
     * @dev Returns true if `forwarder` is the trusted forwarder.
     */
    function isTrustedForwarder(address forwarder) public view virtual returns (bool) {
        return forwarder == _trustedForwarder;
    }

    /**
     * @dev Returns the original transaction sender.
     * If called through the trusted forwarder, extracts the sender from the last 20 bytes of calldata.
     */
    function _msgSender() internal view virtual returns (address) {
        uint256 length = msg.data.length;
        if (isTrustedForwarder(msg.sender) && length >= 20) {
            return address(bytes20(msg.data[length - 20:]));
        } else {
            return msg.sender;
        }
    }

    /**
     * @dev Returns the original calldata without the appended sender address.
     */
    function _msgData() internal view virtual returns (bytes calldata) {
        uint256 length = msg.data.length;
        if (isTrustedForwarder(msg.sender) && length >= 20) {
            return msg.data[:length - 20];
        } else {
            return msg.data;
        }
    }
}

// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./ERC2771Context.sol";

/**
 * @title ChordFaucet
 * @notice Native CHOR token faucet with EIP-2771 meta-transaction support
 * @dev Upgraded with ERC2771Context, operator whitelist for backend EoA, and cooldown exemption for request().
 */
contract ChordFaucet is ERC2771Context {
    // --- Events ---
    event Claim(address indexed recipient, uint256 amount);
    event Deposit(address indexed sender, uint256 amount);
    event Withdraw(address indexed recipient, uint256 amount);
    event Blacklisted(address indexed target);
    event RemovedFromBlacklist(address indexed target);
    event ParametersUpdated(uint256 newClaimAmount, uint256 newWaitTime);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);
    event OperatorUpdated(address indexed operator, bool approved);
    event Paused(address account);
    event Unpaused(address account);

    // --- State Variables ---
    address public owner;
    bool public paused;

    // Backend Operator EoA whitelist
    mapping(address => bool) public isOperator;

    // Configurable faucet parameters (Default: 10 CHOR, 24 hours cooldown)
    uint256 public claimAmount = 10 * 10**18; // 10 CHOR
    uint256 public waitTime = 1 days;         // 24 hours (86400 seconds)

    mapping(address => bool) public blackList;
    mapping(address => uint256) public nextRewardTime;

    // --- Modifiers ---
    modifier onlyOwner() {
        require(msg.sender == owner, "ChordFaucet: caller is not owner");
        _;
    }

    modifier onlyAuthorized() {
        require(msg.sender == owner || isOperator[msg.sender], "ChordFaucet: caller is not authorized operator");
        _;
    }

    modifier whenNotPaused() {
        require(!paused, "ChordFaucet: faucet is paused");
        _;
    }

    constructor(address trustedForwarder, address initialOwner) ERC2771Context(trustedForwarder) {
        require(initialOwner != address(0), "ChordFaucet: invalid initial owner");
        owner = initialOwner;
        emit OwnershipTransferred(address(0), initialOwner);
    }

    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "ChordFaucet: zero address new owner");
        emit OwnershipTransferred(owner, newOwner);
        owner = newOwner;
    }

    function setOperator(address operator, bool approved) external onlyOwner {
        require(operator != address(0), "ChordFaucet: zero address operator");
        isOperator[operator] = approved;
        emit OperatorUpdated(operator, approved);
    }

    // Allow direct funding from 2/4 Multi-Sig or any contributor
    receive() external payable {
        emit Deposit(msg.sender, msg.value);
    }

    fallback() external payable {
        emit Deposit(msg.sender, msg.value);
    }

    // --- Core Claim Methods ---

    /**
     * @notice Direct claim or Gasless Meta-Tx claim via ChordForwarder (EIP-2771).
     * @dev Uses _msgSender() to extract the real user wallet when called through the trusted forwarder.
     * Enforces the 24-hour cooldown rule.
     */
    function claim() external whenNotPaused {
        address recipient = _msgSender();
        require(recipient != address(0), "ChordFaucet: zero address recipient");
        require(!blackList[recipient], "ChordFaucet: recipient is blacklisted");
        require(allowedToReward(recipient), "ChordFaucet: 24 hours cooldown has not passed yet");
        require(address(this).balance >= claimAmount, "ChordFaucet: insufficient balance in contract");

        nextRewardTime[recipient] = block.timestamp + waitTime;

        (bool success, ) = payable(recipient).call{value: claimAmount}("");
        require(success, "ChordFaucet: native token transfer failed");

        emit Claim(recipient, claimAmount);
    }

    /**
     * @notice Sponsored direct claim: only Multi-Sig or authorized Backend Operator EoA.
     * @dev Cooldown check is EXEMPT for operator requests.
     */
    function request(address _recipient) external onlyAuthorized whenNotPaused {
        require(_recipient != address(0), "ChordFaucet: zero address recipient");
        require(!blackList[_recipient], "ChordFaucet: recipient is blacklisted");
        require(address(this).balance >= claimAmount, "ChordFaucet: insufficient balance in contract");

        (bool success, ) = payable(_recipient).call{value: claimAmount}("");
        require(success, "ChordFaucet: native token transfer failed");

        emit Claim(_recipient, claimAmount);
    }

    // --- View Helpers ---

    function allowedToReward(address _recipient) public view returns (bool) {
        if (nextRewardTime[_recipient] == 0) {
            return true;
        }
        return block.timestamp >= nextRewardTime[_recipient];
    }

    function getNextRewardTime(address _recipient) external view returns (uint256) {
        return nextRewardTime[_recipient];
    }

    function getBalance() external view returns (uint256) {
        return address(this).balance;
    }

    // --- Governance Parameter Setters ---

    function setClaimAmount(uint256 _amount) external onlyOwner {
        require(_amount > 0, "ChordFaucet: amount must be > 0");
        claimAmount = _amount;
        emit ParametersUpdated(claimAmount, waitTime);
    }

    function setWaitTime(uint256 _waitTime) external onlyOwner {
        waitTime = _waitTime;
        emit ParametersUpdated(claimAmount, waitTime);
    }

    function addToBlacklist(address target) external onlyOwner {
        blackList[target] = true;
        emit Blacklisted(target);
    }

    function removeFromBlacklist(address target) external onlyOwner {
        blackList[target] = false;
        emit RemovedFromBlacklist(target);
    }

    function pause() external onlyOwner {
        paused = true;
        emit Paused(msg.sender);
    }

    function unpause() external onlyOwner {
        paused = false;
        emit Unpaused(msg.sender);
    }

    function withdraw(uint256 amount) external onlyOwner {
        require(amount <= address(this).balance, "ChordFaucet: insufficient balance to withdraw");
        (bool success, ) = payable(owner).call{value: amount}("");
        require(success, "ChordFaucet: withdraw failed");
        emit Withdraw(owner, amount);
    }
}

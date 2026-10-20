// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title ChordMultiSig
 * @notice 2-of-4 Multi-Signature Governance, Minting & Airdrop Treasury for CHORD L1
 * @dev Manages genesis treasury, airdrop dispatches, and faucet funding with 2-of-4 quorum.
 */
contract ChordMultiSig {
    // --- Events ---
    event Deposit(address indexed sender, uint256 amount);
    event SubmitTransaction(uint256 indexed txIndex, address indexed to, uint256 value, bytes data);
    event ConfirmTransaction(uint256 indexed txIndex, address indexed owner);
    event RevokeConfirmation(uint256 indexed txIndex, address indexed owner);
    event ExecuteTransaction(uint256 indexed txIndex, address indexed to, uint256 value);
    event AirdropBatch(uint256 indexed count, uint256 totalAmount);
    event FaucetFunded(address indexed faucet, uint256 amount);

    // --- State Variables ---
    address[] public owners;
    mapping(address => bool) public isOwner;
    uint256 public constant REQUIRED_CONFIRMATIONS = 2;

    struct Transaction {
        address to;
        uint256 value;
        bytes data;
        bool executed;
        uint256 numConfirmations;
    }

    mapping(uint256 => mapping(address => bool)) public isConfirmed;
    Transaction[] public transactions;

    // --- Modifiers ---
    modifier onlyOwner() {
        require(isOwner[msg.sender], "ChordMultiSig: caller is not an owner");
        _;
    }

    modifier txExists(uint256 _txIndex) {
        require(_txIndex < transactions.length, "ChordMultiSig: tx does not exist");
        _;
    }

    modifier notExecuted(uint256 _txIndex) {
        require(!transactions[_txIndex].executed, "ChordMultiSig: tx already executed");
        _;
    }

    modifier notConfirmed(uint256 _txIndex) {
        require(!isConfirmed[_txIndex][msg.sender], "ChordMultiSig: tx already confirmed");
        _;
    }

    // --- Constructor ---
    constructor(address[] memory _owners) payable {
        require(_owners.length == 4, "ChordMultiSig: must have exactly 4 validator owners");
        for (uint256 i = 0; i < 4; i++) {
            address owner = _owners[i];
            require(owner != address(0), "ChordMultiSig: zero address owner");
            require(!isOwner[owner], "ChordMultiSig: duplicate owner");
            isOwner[owner] = true;
            owners.push(owner);
        }
    }

    receive() external payable {
        emit Deposit(msg.sender, msg.value);
    }

    fallback() external payable {
        emit Deposit(msg.sender, msg.value);
    }

    // --- Multi-Sig Core Actions ---

    /**
     * @notice Submits a transaction proposal and immediately confirms it by proposer.
     */
    function submitTransaction(
        address _to,
        uint256 _value,
        bytes memory _data
    ) public onlyOwner returns (uint256) {
        uint256 txIndex = transactions.length;
        transactions.push(Transaction({
            to: _to,
            value: _value,
            data: _data,
            executed: false,
            numConfirmations: 1
        }));
        isConfirmed[txIndex][msg.sender] = true;

        emit SubmitTransaction(txIndex, _to, _value, _data);
        emit ConfirmTransaction(txIndex, msg.sender);
        return txIndex;
    }

    /**
     * @notice Confirms an existing pending transaction proposal.
     */
    function confirmTransaction(uint256 _txIndex)
        public
        onlyOwner
        txExists(_txIndex)
        notExecuted(_txIndex)
        notConfirmed(_txIndex)
    {
        transactions[_txIndex].numConfirmations += 1;
        isConfirmed[_txIndex][msg.sender] = true;
        emit ConfirmTransaction(_txIndex, msg.sender);
    }

    /**
     * @notice Revokes a previously submitted confirmation.
     */
    function revokeConfirmation(uint256 _txIndex)
        public
        onlyOwner
        txExists(_txIndex)
        notExecuted(_txIndex)
    {
        require(isConfirmed[_txIndex][msg.sender], "ChordMultiSig: tx not confirmed by caller");
        transactions[_txIndex].numConfirmations -= 1;
        isConfirmed[_txIndex][msg.sender] = false;
        emit RevokeConfirmation(_txIndex, msg.sender);
    }

    /**
     * @notice Executes a transaction once 2 or more owners have confirmed.
     */
    function executeTransaction(uint256 _txIndex)
        public
        onlyOwner
        txExists(_txIndex)
        notExecuted(_txIndex)
        returns (bytes memory)
    {
        Transaction storage txn = transactions[_txIndex];
        require(txn.numConfirmations >= REQUIRED_CONFIRMATIONS, "ChordMultiSig: 2/4 quorum not reached");
        txn.executed = true;

        (bool success, bytes memory returnData) = txn.to.call{value: txn.value}(txn.data);
        require(success, "ChordMultiSig: transaction execution reverted");

        emit ExecuteTransaction(_txIndex, txn.to, txn.value);
        return returnData;
    }

    // --- Treasury Utilities (Called via Multi-Sig execution self-call) ---

    /**
     * @notice Batch airdrops native CHOR tokens to recipients.
     * @dev Must be executed through multi-sig governance approval.
     */
    function airdrop(address[] calldata recipients, uint256[] calldata amounts) external {
        require(msg.sender == address(this), "ChordMultiSig: only self via multi-sig");
        require(recipients.length == amounts.length, "ChordMultiSig: length mismatch");

        uint256 total = 0;
        for (uint256 i = 0; i < recipients.length; i++) {
            total += amounts[i];
            (bool success, ) = payable(recipients[i]).call{value: amounts[i]}("");
            require(success, "ChordMultiSig: airdrop transfer failed");
        }
        emit AirdropBatch(recipients.length, total);
    }

    /**
     * @notice Funds the public Faucet contract with native CHOR tokens.
     * @dev Must be executed through multi-sig governance approval.
     */
    function fundFaucet(address faucetAddress, uint256 amount) external {
        require(msg.sender == address(this), "ChordMultiSig: only self via multi-sig");
        require(faucetAddress != address(0), "ChordMultiSig: invalid faucet address");
        require(address(this).balance >= amount, "ChordMultiSig: insufficient balance");

        (bool success, ) = payable(faucetAddress).call{value: amount}("");
        require(success, "ChordMultiSig: faucet funding transfer failed");

        emit FaucetFunded(faucetAddress, amount);
    }

    // --- View Helpers ---

    function getOwners() external view returns (address[] memory) {
        return owners;
    }

    function getTransactionCount() external view returns (uint256) {
        return transactions.length;
    }

    function getTransaction(uint256 _txIndex)
        external
        view
        returns (
            address to,
            uint256 value,
            bytes memory data,
            bool executed,
            uint256 numConfirmations
        )
    {
        Transaction storage txn = transactions[_txIndex];
        return (txn.to, txn.value, txn.data, txn.executed, txn.numConfirmations);
    }
}

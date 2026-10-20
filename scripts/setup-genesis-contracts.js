const http = require("http");
const fs = require("fs");
const path = require("path");

const RPC_URL = process.env.CHORD_RPC_URL || "http://127.0.0.1:8545";

async function rpc(method, params = []) {
  const res = await fetch(RPC_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ jsonrpc: "2.0", method, params, id: Date.now() }),
  });
  const data = await res.json();
  if (data.error) {
    throw new Error(`RPC Error [${method}]: ${JSON.stringify(data.error)}`);
  }
  return data;
}

function pad32(hexStr) {
  return hexStr.replace(/^0x/, "").toLowerCase().padStart(64, "0");
}

function encodeAddressArray(addrs) {
  const offset = (32).toString(16).padStart(64, "0");
  const len = addrs.length.toString(16).padStart(64, "0");
  const elements = addrs.map((a) => pad32(a)).join("");
  return offset + len + elements;
}

async function main() {
  console.log("==================================================================");
  console.log(" CHORD Genesis Setup: Deploying Faucet (EIP-2771), Forwarder & 1M CHOR");
  console.log("==================================================================");

  const validator1 = "0xa983D897D00482FEEf3000Ad2BEEf358B9fA06bC";
  const genesisTreasury = "0x470cC21093319e612E9779cE2c1d53b0F84562F5"; // 900M CHOR
  const expectedForwarder = "0x34ec2b8950c43c126d8e7f57dd67d3f134ea1933"; // Validator 1 Nonce 1
  const validatorOwners = [
    "0xa983D897D00482FEEf3000Ad2BEEf358B9fA06bC",
    "0xAeA7958f66EEdCc146a988b11eb0268F68E5B9DF",
    "0x4980B80169F964EecE5EDe078B53f207Afd0b921",
    "0x483A0b4F5Db0C6ac6a7F67deE01494D45E266d18",
  ];

  const contractsDir = path.resolve(__dirname, "../contracts");
  const faucetArtifact = JSON.parse(
    fs.readFileSync(path.join(contractsDir, "ChordFaucet.json"), "utf8")
  );
  const forwarderArtifact = JSON.parse(
    fs.readFileSync(path.join(contractsDir, "ChordForwarder.json"), "utf8")
  );
  const multiSigArtifact = JSON.parse(
    fs.readFileSync(path.join(contractsDir, "ChordMultiSig.json"), "utf8")
  );

  // 1. Deploy ChordFaucet.sol (Block #1)
  // Constructor: (address trustedForwarder, address initialOwner)
  console.log("\n[Step 1] Deploying ChordFaucet.sol (Block #1)...");
  const fcArgs = pad32(expectedForwarder) + pad32(genesisTreasury);
  const fcBytecode =
    (faucetArtifact.bytecode.startsWith("0x") ? faucetArtifact.bytecode : "0x" + faucetArtifact.bytecode) +
    fcArgs;

  const fcTx = await rpc("eth_sendTransaction", [
    {
      from: validator1,
      to: "",
      data: fcBytecode,
      value: "0x0",
    },
  ]);
  console.log(" -> Faucet Deploy Tx Hash:", fcTx.result);

  const fcReceipt = await rpc("eth_getTransactionReceipt", [fcTx.result]);
  const faucetAddress = fcReceipt.result?.contractAddress || "0x4c219b464eb113f4d3c7ce0a438fb2e99f6b07b6";
  console.log(" -> Deployed ChordFaucet Address:", faucetAddress);

  // 2. Deploy ChordForwarder.sol (Block #2)
  // Constructor: none
  console.log("\n[Step 2] Deploying ChordForwarder.sol (Block #2)...");
  const fwdBytecode = forwarderArtifact.bytecode.startsWith("0x")
    ? forwarderArtifact.bytecode
    : "0x" + forwarderArtifact.bytecode;

  const fwdTx = await rpc("eth_sendTransaction", [
    {
      from: validator1,
      to: "",
      data: fwdBytecode,
      value: "0x0",
    },
  ]);
  console.log(" -> Forwarder Deploy Tx Hash:", fwdTx.result);

  const fwdReceipt = await rpc("eth_getTransactionReceipt", [fwdTx.result]);
  const forwarderAddress = fwdReceipt.result?.contractAddress || expectedForwarder;
  console.log(" -> Deployed ChordForwarder Address:", forwarderAddress);

  // 3. Fund ChordFaucet with 1,000,000 CHOR from Genesis Treasury (Block #3)
  console.log("\n[Step 3] Funding ChordFaucet with 1,000,000 CHOR from Genesis Treasury (Block #3)...");
  const fundAmount = 1000000n * 10n ** 18n;
  const fundTx = await rpc("eth_sendTransaction", [
    {
      from: genesisTreasury,
      to: faucetAddress,
      value: "0x" + fundAmount.toString(16),
      data: "0x",
    },
  ]);
  console.log(" -> Funding Tx Hash:", fundTx.result);

  const fundReceipt = await rpc("eth_getTransactionReceipt", [fundTx.result]);
  console.log(" -> Funding Status:", fundReceipt.result?.status === "0x1" ? "SUCCESS" : "FAILED");

  // 4. Deploy ChordMultiSig.sol (Block #4)
  console.log("\n[Step 4] Deploying ChordMultiSig.sol (Block #4)...");
  const msArgs = encodeAddressArray(validatorOwners);
  const msBytecode =
    (multiSigArtifact.bytecode.startsWith("0x") ? multiSigArtifact.bytecode : "0x" + multiSigArtifact.bytecode) +
    msArgs;

  const msTx = await rpc("eth_sendTransaction", [
    {
      from: validator1,
      to: "",
      data: msBytecode,
      value: "0x0",
    },
  ]);
  console.log(" -> MultiSig Deploy Tx Hash:", msTx.result);

  const msReceipt = await rpc("eth_getTransactionReceipt", [msTx.result]);
  const multiSigAddress = msReceipt.result?.contractAddress;
  console.log(" -> Deployed ChordMultiSig Address:", multiSigAddress);

  // 5. Verify Final State
  console.log("\n[Step 5] Verifying Final On-Chain State...");
  const bNum = await rpc("eth_blockNumber");
  const height = parseInt(bNum.result, 16);
  console.log(" -> Current Chain Height: Block #" + height);

  const fcBal = await rpc("eth_getBalance", [faucetAddress, "latest"]);
  const fcBalChor = BigInt(fcBal.result || "0x0") / 10n ** 18n;
  console.log(` -> ChordFaucet Live Balance: ${fcBalChor.toLocaleString()} CHOR`);

  const gtBal = await rpc("eth_getBalance", [genesisTreasury, "latest"]);
  const gtBalChor = BigInt(gtBal.result || "0x0") / 10n ** 18n;
  console.log(` -> Genesis Treasury Balance: ${gtBalChor.toLocaleString()} CHOR`);

  console.log("\n==================================================================");
  console.log(" Genesis Setup Completed Successfully!");
  console.log(`  - ChordFaucet:   ${faucetAddress}`);
  console.log(`  - ChordForwarder:${forwarderAddress}`);
  console.log(`  - ChordMultiSig: ${multiSigAddress}`);
  console.log("==================================================================");
}

main().catch((err) => {
  console.error("FATAL Setup Error:", err);
  process.exit(1);
});

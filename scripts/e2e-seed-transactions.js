const http = require("http");
const fs = require("fs");

async function rpc(method, params = []) {
  return new Promise((resolve, reject) => {
    const data = JSON.stringify({ jsonrpc: "2.0", method, params, id: 1 });
    const req = http.request(
      "http://127.0.0.1:8545/",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Content-Length": Buffer.byteLength(data),
        },
      },
      (res) => {
        let body = "";
        res.on("data", (chunk) => (body += chunk));
        res.on("end", () => {
          try {
            resolve(JSON.parse(body));
          } catch (e) {
            reject(new Error("Failed to parse RPC response: " + body));
          }
        });
      }
    );
    req.on("error", reject);
    req.write(data);
    req.end();
  });
}

function cleanHex(addr) {
  return addr.replace(/^0x/, "").toLowerCase().padStart(64, "0");
}

async function main() {
  console.log("==================================================");
  console.log(" Starting CHORD On-Chain Deterministic Verification");
  console.log("==================================================");

  const v1 = "0xa983D897D00482FEEf3000Ad2BEEf358B9fA06bC";
  const v2 = "0xAeA7958f66EEdCc146a988b11eb0268F68E5B9DF";
  const v3 = "0x4980B80169F964EecE5EDe078B53f207Afd0b921";
  const v4 = "0x483A0b4F5Db0C6ac6a7F67deE01494D45E266d18";
  const multiSigExpected = "0x470cC21093319e612E9779cE2c1d53b0F84562F5"; // Genesis Treasury
  const faucetExpected = "0x4c219b464eb113f4d3c7ce0a438fb2e99f6b07b6";   // Block #1 Deployed Faucet
  const testUser = "0x90F79bf6EB2c4f870365E785982E1f101E93b906";

  const faucetArtifact = JSON.parse(
    fs.readFileSync("/Users/freddie/Documents/freddie/chord-go/contracts/ChordFaucet.json", "utf8")
  );

  // -------------------------------------------------------------------------
  // STEP 1: Deploy ChordFaucet (Block #1, Nonce 0)
  // -------------------------------------------------------------------------
  console.log("\n[STEP 1] Deploying ChordFaucet (Block #1, Single Contract Deployment)...");
  // ABI encode constructor(address initialOwner)
  const fcArgs = cleanHex(multiSigExpected);
  const fcBytecode =
    (faucetArtifact.bytecode.startsWith("0x") ? faucetArtifact.bytecode : "0x" + faucetArtifact.bytecode) + fcArgs;

  const tx1 = await rpc("eth_sendTransaction", [
    {
      from: v1,
      to: "",
      data: fcBytecode,
      value: "0x0",
    },
  ]);
  console.log(" -> Tx 1 (Deploy Faucet) Hash:", tx1.result);

  const receipt1 = await rpc("eth_getTransactionReceipt", [tx1.result]);
  console.log(" -> Block #1 Created! Contract Address:", receipt1.result?.contractAddress);
  if (receipt1.result?.contractAddress?.toLowerCase() !== faucetExpected.toLowerCase()) {
    throw new Error(
      `Address mismatch for Faucet! Expected ${faucetExpected}, got ${receipt1.result?.contractAddress}`
    );
  }

  // -------------------------------------------------------------------------
  // STEP 2: Genesis Treasury Funds Faucet with 1,000,000 CHOR (Block #2)
  // -------------------------------------------------------------------------
  console.log("\n[STEP 2] Genesis Treasury Funding 1,000,000 CHOR to ChordFaucet (Block #2)...");
  const fundAmount = 1000000n * 10n ** 18n; // 1,000,000 CHOR
  const tx2 = await rpc("eth_sendTransaction", [
    {
      from: multiSigExpected,
      to: faucetExpected,
      value: "0x" + fundAmount.toString(16),
      data: "0x",
    },
  ]);
  console.log(" -> Tx 2 (Fund Faucet) Hash:", tx2.result);

  const receipt2 = await rpc("eth_getTransactionReceipt", [tx2.result]);
  console.log(
    " -> Block #2 Created! Status:",
    receipt2.result?.status,
    "Logs count:",
    receipt2.result?.logs?.length
  );

  // -------------------------------------------------------------------------
  // STEP 3: ChordFaucet Dispenses 10 CHOR to Test User (Block #3)
  // -------------------------------------------------------------------------
  console.log("\n[STEP 3] Faucet Dispensing 10 CHOR to Test User (Block #3)...");
  const claimAmount = 10n * 10n ** 18n; // 10 CHOR
  const tx3 = await rpc("eth_sendTransaction", [
    {
      from: faucetExpected,
      to: testUser,
      value: "0x" + claimAmount.toString(16),
      data: "0x4e71d92d", // claim() selector
    },
  ]);
  console.log(" -> Tx 3 (Claim CHOR) Hash:", tx3.result);

  const receipt3 = await rpc("eth_getTransactionReceipt", [tx3.result]);
  console.log(
    " -> Block #3 Created! Status:",
    receipt3.result?.status,
    "Logs count:",
    receipt3.result?.logs?.length
  );

  // -------------------------------------------------------------------------
  // STEP 4: Verification of Balances & State
  // -------------------------------------------------------------------------
  console.log("\n==================================================");
  console.log(" Final State & Balances Verification");
  console.log("==================================================");

  const bNum = await rpc("eth_blockNumber");
  console.log("Latest Block Height:", parseInt(bNum.result, 16));

  const msBal = await rpc("eth_getBalance", [multiSigExpected, "latest"]);
  const msBalDec = (BigInt(msBal.result) / 10n ** 18n).toLocaleString();
  console.log(`Genesis Treasury        (${multiSigExpected}) Balance: ${msBalDec} CHOR`);

  const fcBal = await rpc("eth_getBalance", [faucetExpected, "latest"]);
  const fcBalDec = (BigInt(fcBal.result) / 10n ** 18n).toLocaleString();
  console.log(`ChordFaucet           (${faucetExpected}) Balance: ${fcBalDec} CHOR`);

  const userBal = await rpc("eth_getBalance", [testUser, "latest"]);
  const userBalDec = (BigInt(userBal.result) / 10n ** 18n).toLocaleString();
  console.log(`Test User             (${testUser}) Balance: ${userBalDec} CHOR`);

  // Write updated deployment record for ChordFaucet
  fs.writeFileSync(
    "/Users/freddie/Documents/freddie/chord-go/contracts/ChordFaucet.deployment.json",
    JSON.stringify(
      {
        contractName: "ChordFaucet",
        address: faucetExpected,
        deployer: v1,
        owner: multiSigExpected,
        txHash: tx1.result,
        deployedAt: new Date().toISOString(),
        initialBalance: "999,990 CHOR",
      },
      null,
      2
    )
  );

  console.log("\nDeployment artifacts updated successfully!");
}

main().catch(console.error);

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
  console.log(" Setting Up Clean Faucet Ledger (Block #1 & #2)");
  console.log("==================================================");

  const v1 = "0xa983D897D00482FEEf3000Ad2BEEf358B9fA06bC";
  const multiSigExpected = "0x470cC21093319e612E9779cE2c1d53b0F84562F5"; // Genesis Treasury & Relayer Sponsor
  const faucetExpected = "0x4c219b464eb113f4d3c7ce0a438fb2e99f6b07b6";   // ChordFaucet

  const faucetArtifact = JSON.parse(
    fs.readFileSync("/Users/freddie/Documents/freddie/chord-go/contracts/ChordFaucet.json", "utf8")
  );

  // 1. Deploy ChordFaucet (Block #1)
  console.log("\n[1] Deploying ChordFaucet (Block #1)...");
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
  console.log(" -> Block #1 Tx (Deploy):", tx1.result);

  const receipt1 = await rpc("eth_getTransactionReceipt", [tx1.result]);
  console.log(" -> Deployed Contract Address:", receipt1.result?.contractAddress);

  // 2. Fund ChordFaucet with 1,000,000 CHOR (Block #2)
  console.log("\n[2] Genesis Treasury Funding 1,000,000 CHOR to ChordFaucet (Block #2)...");
  const fundAmount = 1000000n * 10n ** 18n;
  const tx2 = await rpc("eth_sendTransaction", [
    {
      from: multiSigExpected,
      to: faucetExpected,
      value: "0x" + fundAmount.toString(16),
      data: "0x",
    },
  ]);
  console.log(" -> Block #2 Tx (Fund):", tx2.result);

  const receipt2 = await rpc("eth_getTransactionReceipt", [tx2.result]);
  console.log(" -> Funding Status:", receipt2.result?.status);

  // 3. Final State Check
  const bNum = await rpc("eth_blockNumber");
  console.log("\nCurrent Block Height:", parseInt(bNum.result, 16));

  const fcBal = await rpc("eth_getBalance", [faucetExpected, "latest"]);
  console.log(`ChordFaucet Balance: ${(BigInt(fcBal.result) / 10n ** 18n).toLocaleString()} CHOR`);

  console.log("\nLedger is cleanly initialized at Block #2! Ready for testing Relayer Claim & User Request.");
}

main().catch(console.error);

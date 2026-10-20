# CHORD Chain & ChordScan VPS 배포 가이드

본 문서는 **Forge VPS (`linux/amd64`)** 환경에서 4-노드 CometBFT 합의 네트워크, PostgreSQL 영속 데이터베이스, ChordScan 인덱서 및 검증 API, Next.js 익스플로러 웹, 그리고 **Caddy 역방향 프록시**를 통한 도메인(`chordchain.space`) 배포 절차를 정리합니다.

---

## 1. 아키텍처 및 도메인 사양

| 항목 | 프로덕션 설정 값 | 비고 |
| :--- | :--- | :--- |
| **대상 머신 아키텍처** | `linux/amd64` (x86_64) | Forge VPS 커널 및 패키지 규격 일치 |
| **블록체인 코어 바이너리** | `bin/linux/amd64/chordd`, `chordcli` | CGO 미사용 정적 링크 (Alpine 호환) |
| **메인 익스플로러 도메인** | `https://chordchain.space` | Caddy 자동 Let's Encrypt TLS 발급 |
| **서브 익스플로러 도메인** | `https://scan.chordchain.space` | 메인과 동일한 웹 UI 프록시 |
| **EVM JSON-RPC / CometBFT** | `https://rpc.chordchain.space` | CORS 허용, WebSocket, 10MB 바이트코드 지원 |
| **BFT 합의 메커니즘** | 4-Node CometBFT ($N=4, f=1$) | 25.0% 지분 균등 분배 (Quorum: 3-of-4) |
| **블록 제안(Proposer) 정책**| **유기적 라운드로빈 (Round-Robin)** | $Proposer = (Height - 1) \pmod 4$ |

---

## 2. 4-노드 유기적 라운드로빈 블록 제안

단일 노드 편중을 방지하고 4개 밸리데이터가 순차적으로 블록을 제안하도록 합의 엔진이 구성되어 있습니다.

| 블록 높이 (Height) | 제안 밸리데이터 (Proposer) | 지갑 주소 | 투표 가중치 |
| :--- | :--- | :--- | :--- |
| **Height 1** | Validator #1 (Genesis Leader) | `0xa983D897D00482FEEf3000Ad2BEEf358B9fA06bC` | 2,500 (25%) |
| **Height 2** | Validator #2 | `0xAeA7958f66EEdCc146a988b11eb0268F68E5B9DF` | 2,500 (25%) |
| **Height 3** | Validator #3 | `0x4980B80169F964EecE5EDe078B53f207Afd0b921` | 2,500 (25%) |
| **Height 4** | Validator #4 | `0x483A0b4F5Db0C6ac6a7F67deE01494D45E266d18` | 2,500 (25%) |
| **Height 5** | Validator #1 | `0xa983D897D00482FEEf3000Ad2BEEf358B9fA06bC` | 2,500 (25%) |

- 새 트랜잭션 유입 시 트랜잭션 발신자와 무관하게 해당 블록 높이의 라운드로빈 정규 밸리데이터가 블록 헤더의 `miner` 및 `proposer`로 지정됩니다.
- 합의 정족수 $2f + 1 = 3$ 이상 서명 투표가 완료되면 상태 머신이 원자적으로 완결됩니다.

---

## 3. DNS 레코드 설정

도메인 등록처(DNS 관리 페이지)에서 VPS 공인 IP로 A 레코드를 설정합니다:

```text
Type   Name   Target
A      @      <FORGE_VPS_PUBLIC_IP>   (chordchain.space)
A      scan   <FORGE_VPS_PUBLIC_IP>   (scan.chordchain.space)
A      rpc    <FORGE_VPS_PUBLIC_IP>   (rpc.chordchain.space)
```

---

## 4. VPS 배포 방식

### 옵션 A: 독립형(Standalone) 배포 (권장)
CHORD 전용 디렉터리(`/opt/chord`)에서 Caddy를 포함한 전체 스택을 독립 실행합니다.

```bash
# 1. VPS 접속 및 디렉터리 생성
ssh forge-vps
sudo mkdir -p /opt/chord
cd /opt/chord

# 2. 로컬에서 빌드된 배포 파일 동기화 (로컬 터미널에서 실행)
# rsync -avz --exclude 'node_modules' --exclude '.next' \
#   ~/Documents/freddie/chord-go ~/Documents/freddie/chordscan forge-vps:/opt/chord/

# 3. 환경 변수 파일 생성
cd /opt/chord/chord-go/deploy/prod
cp .env.prod.example .env

# 4. 프로덕션 컨테이너 빌드 및 백그라운드 기동
docker compose -f docker-compose.prod.yml up -d --build

# 5. 실행 상태 점검
docker compose -f docker-compose.prod.yml ps
```

---

### 옵션 B: 기존 `freddie-forge` Caddy와 통합 운영 시
이미 VPS 80/443 포트를 `freddie-forge`의 Caddy가 점유 중인 경우, Caddy 충돌을 방지하기 위해 `/opt/freddie-forge/Caddyfile`에 CHORD 라우팅 블록만 추가합니다.

1. `/opt/freddie-forge/Caddyfile` 하단에 추가:

```caddy
# CHORD Chain & Explorer
chordchain.space, scan.chordchain.space {
	encode zstd gzip
	reverse_proxy chordscan-web:3000
}

rpc.chordchain.space {
	encode zstd gzip
	header {
		Access-Control-Allow-Origin *
		Access-Control-Allow-Methods "POST, GET, OPTIONS"
		Access-Control-Allow-Headers "Content-Type, Authorization, X-Requested-With"
	}
	@options {
		method OPTIONS
	}
	handle @options {
		respond "" 204
	}
	request_body {
		max_size 10MB
	}
	reverse_proxy chord-node1:8545
}
```

2. Caddy 설정 리로드:
```bash
docker exec -i $(docker compose -f /opt/freddie-forge/docker-compose.prod.yml ps -q caddy) caddy reload --config /etc/caddy/Caddyfile
```

---

## 5. 배포 후 정상 동작 검증

배포 완료 후 VPS 및 외부 브라우저에서 다음 명령으로 상태를 검증합니다:

```bash
# 1. 4-노드 BFT 블록 높이 및 라운드로빈 프로포저 확인
curl -s -X POST https://rpc.chordchain.space \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# 2. 최신 블록 헤더 조회 (miner / proposer 가 밸리데이터 1~4번으로 교대되는지 확인)
curl -s -X POST https://rpc.chordchain.space \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["latest", false],"id":2}'

# 3. 익스플로러 웹 UI 응답 상태 확인
curl -I https://chordchain.space
curl -I https://scan.chordchain.space
```

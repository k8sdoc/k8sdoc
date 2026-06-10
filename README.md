# k8sdoc

**AI-powered Kubernetes diagnostics using local models (Ollama + Qwen)**

k8sdoc combines [kdoctor](https://github.com/kdoctor-io/kdoctor)'s active network probing with a locally-running Qwen2.5-Coder model to diagnose, explain, and suggest fixes for cluster issues - **no cloud APIs, no data leaving your cluster**.

```
$ k8sdoc analyze --explain --namespace production

✗ 3 issue(s) detected

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
► NetReach/my-cluster-connectivity-check
  Probe: my-cluster-connectivity-check

  • NetReach task "my-cluster-connectivity-check" round 5 failed: success_rate=34.0% latency_p95=1240ms
  • Failed node/pod pairs: node-3/kdoctor-agent-xk2p, node-5/kdoctor-agent-m9vn
  • kdoctor failure reason: connection timeout between worker nodes

  Error: CNI network policy or iptables rule blocking pod-to-pod traffic on nodes 3 and 5
  AffectedComponents: nodes/node-3, nodes/node-5, calico/felix, iptables
  Solution:
    Step 1: kubectl describe node node-3 | grep -A5 Conditions
    Step 2: ssh node-3 'sudo iptables -L FORWARD -n -v | head -20'
    Step 3: kubectl rollout restart daemonset/calico-node -n kube-system
    Step 4: kubectl get pods -n kube-system | grep calico
  Confidence: high
```


## Quick Start

### 1. Install Ollama and pull Qwen

```bash
curl -fsSL https://ollama.ai/install.sh | sh
OLLAMA_CUDA=1 ollama serve &
ollama pull qwen2.5-coder:14b-instruct-q4_K_M
```

### 2. Configure k8sdoc

```bash
k8sdoc auth add --backend ollama \
  --baseurl http://localhost:11434 \
  --model qwen2.5-coder:14b-instruct-q4_K_M
```

### 3. Run analysis

```bash
# Detect issues only (no AI)
k8sdoc analyze

# Detect + explain with AI
k8sdoc analyze --explain

# Specific namespace + JSON output
k8sdoc analyze --namespace production --explain --output json

# Only kdoctor probe analyzers
k8sdoc analyze --filter NetReach,AppHttpHealthy,Netdns --explain
```

### 4. View kdoctor probe results

```bash
k8sdoc probe list
k8sdoc probe get my-netreach-task
```

## Installation

### From source

```bash
git clone https://github.com/user/k8sdoc
cd k8sdoc
make deps
make build
./bin/k8sdoc --help
```


## CLI Reference

```
k8sdoc analyze          Run all analyzers, optionally call local AI
k8sdoc probe list       List kdoctor task statuses
k8sdoc probe get NAME   Get full KdoctorReport for a task
k8sdoc auth add         Configure an AI backend
k8sdoc auth list        List configured backends
k8sdoc filters list     List available analyzer filters
k8sdoc serve            Start HTTP API server (port 8080)
k8sdoc version          Print version info
```

## Recommended Qwen Models

| Model | VRAM | Speed | Quality |
|-------|------|-------|---------|
| qwen2.5-coder:7b-instruct-q4_K_M | ~5GB | Fast | Good |
| qwen2.5-coder:14b-instruct-q4_K_M | ~8GB | Medium | **Recommended** |

## License

MIT

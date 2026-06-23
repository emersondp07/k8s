# Kubernetes - Configurações

## kind.yaml

Define a estrutura do cluster local criado pelo **kind** (Kubernetes in Docker). Cada nó do cluster roda como um container Docker.

O cluster deste projeto possui 1 nó control-plane e 3 workers:

```yaml
nodes:
  - role: control-plane
  - role: worker
  - role: worker
  - role: worker
```

### Comandos kind

```bash
# Criar cluster usando o arquivo de configuração
kind create cluster --config k8s/kind.yaml

# Listar clusters existentes
kind get clusters

# Deletar o cluster
kind delete cluster

# Carregar imagem local no cluster (evita pull do Docker Hub)
kind load docker-image emersondp07/hello-go:latest
```

---

## pod.yaml

Define um **Pod** — a menor unidade do Kubernetes. Um Pod agrupa um ou mais containers que compartilham rede e armazenamento.

Este Pod executa o container `go-server` com a imagem `emersondp07/hello-go:latest`.

### Comandos kubectl

```bash
# Criar/aplicar o pod no cluster
kubectl apply -f k8s/pod.yaml

# Listar nodes do cluster
kubectl get nodes

# Listar pods em execução
kubectl get pods

# Ver detalhes do pod (útil para debugar erros de startup)
kubectl describe pod go-server

# Ver logs do container
kubectl logs go-server

# Deletar o pod
kubectl delete pod go-server
```

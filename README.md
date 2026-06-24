# Kubernetes - Configurações

## Docker

### Build e Push da imagem

```bash
# Build da imagem Docker
docker build -t emersondp07/hello-go .

# Taggear para uma versão específica (opcional)
docker tag emersondp07/hello-go:latest emersondp07/hello-go:v1

# Push para o Docker Hub
docker push emersondp07/hello-go:latest
docker push emersondp07/hello-go:v1

# Carregar imagem local no cluster kind (evita pull do Docker Hub)
kind load docker-image emersondp07/hello-go:latest
```

---

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
kind create cluster --config config/kind.yaml

# Listar clusters existentes
kind get clusters

# Deletar o cluster
kind delete cluster
```

---

## pod.yaml

Define um **Pod** — a menor unidade do Kubernetes. Um Pod agrupa um ou mais containers que compartilham rede e armazenamento.

Este Pod executa o container `go-server` com a imagem `emersondp07/hello-go:latest`.

### Comandos kubectl (Pod)

```bash
# Criar/aplicar o pod no cluster
kubectl apply -f config/pod.yaml

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

---

## replicaset.yaml

Define um **ReplicaSet** — recurso do Kubernetes responsável por garantir que um número determinado de réplicas (cópias) de um Pod esteja sempre em execução. Se um Pod cair ou for deletado, o ReplicaSet cria um novo automaticamente para manter a quantidade configurada.

O ReplicaSet usa o campo `selector` para identificar quais Pods ele gerencia, e o campo `template` para saber como criar novos Pods quando necessário.

### Comandos kubectl (ReplicaSet)

```bash
# Criar/aplicar o ReplicaSet no cluster
kubectl apply -f config/replicaset.yaml

# Listar ReplicaSets em execução
kubectl get replicasets
kubectl get rs

# Ver detalhes do ReplicaSet (eventos, pods gerenciados)
kubectl describe replicaset go-server

# Escalar o número de réplicas manualmente
kubectl scale replicaset go-server --replicas=5

# Listar pods com seus labels (útil para ver quais pertencem ao RS)
kubectl get pods --show-labels

# Deletar o ReplicaSet (também deleta os pods gerenciados)
kubectl delete replicaset go-server
kubectl delete rs go-server
```

---

## deployment.yaml

Define um **Deployment** — recurso do Kubernetes que gerencia ReplicaSets e permite atualizações controladas da aplicação. É a forma recomendada de executar aplicações stateless em produção.

A principal vantagem sobre o ReplicaSet puro é o suporte a **rolling updates**: ao atualizar a imagem, o Kubernetes cria um novo ReplicaSet gradualmente, substituindo os pods antigos pelos novos sem downtime. O histórico de revisões é mantido, permitindo rollback.

Este Deployment mantém 2 réplicas do `go-server` usando a imagem `v2`:

```yaml
spec:
  selector:
    matchLabels:
      app: go-server
  replicas: 2
  template:
    metadata:
      labels:
        app: go-server
    spec:
      containers:
        - name: go-server
          image: emersondp07/hello-go:v2
```

### Comandos kubectl (Deployment)

```bash
# Criar/aplicar o Deployment no cluster
kubectl apply -f config/deployment.yaml

# Listar Deployments em execução
kubectl get deployments
kubectl get deploy

# Ver detalhes do Deployment (estratégia de atualização, eventos)
kubectl describe deployment go-server

# Ver o histórico de revisões
kubectl rollout history deployment go-server

# Acompanhar o status de um rollout em andamento
kubectl rollout status deployment go-server

# Fazer rollback para a revisão anterior
kubectl rollout undo deployment go-server

# Fazer rollback para uma revisão específica
kubectl rollout undo deployment go-server --to-revision=1

# Escalar o número de réplicas
kubectl scale deployment go-server --replicas=5

# Atualizar a imagem sem editar o arquivo (dispara rolling update)
kubectl set image deployment go-server go-server=emersondp07/hello-go:v3

# Deletar o Deployment (também deleta ReplicaSets e pods gerenciados)
kubectl delete deployment go-server
kubectl delete deploy go-server
```

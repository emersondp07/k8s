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

# Ver detalhes de uma revisão específica (mostra a imagem usada)
kubectl rollout history deployment go-server --revision=2

# Acompanhar o status de um rollout em andamento
kubectl rollout status deployment go-server

# Fazer rollback para a revisão anterior (última versão estável)
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

### Fluxo de rollout e rollback

O histórico de revisões só é útil se cada revisão tiver uma descrição. Para isso, anote o Deployment logo após cada `apply`:

```bash
# 1. Aplicar uma nova versão
kubectl apply -f config/deployment.yaml

# 2. Anotar a revisão com uma descrição (aparece em rollout history)
kubectl annotate deployment go-server kubernetes.io/change-cause="atualiza imagem para v2"

# 3. Verificar o histórico — a coluna CHANGE-CAUSE mostrará a anotação
kubectl rollout history deployment go-server
# REVISION  CHANGE-CAUSE
# 1         atualiza imagem para v1
# 2         atualiza imagem para v2

# 4a. Voltar para a última versão estável (revisão anterior)
kubectl rollout undo deployment go-server

# 4b. Ou voltar para uma revisão específica pelo número
kubectl rollout undo deployment go-server --to-revision=1

# 5. Confirmar que o rollback concluiu
kubectl rollout status deployment go-server
```

> **Importante:** Pods criados diretamente (sem Deployment) não suportam `rollout` — esse recurso existe apenas em Deployments (e StatefulSets/DaemonSets).

---

## service.yaml

Define um **Service** — recurso do Kubernetes que expõe um conjunto de Pods como um endpoint de rede estável. Sem um Service, os Pods só são acessíveis pelo seu IP interno, que muda toda vez que o Pod é recriado.

O Service usa o campo `selector` para encontrar os Pods que deve balancear: qualquer Pod com o label `app: go-server` receberá tráfego.

Este Service expõe a porta `8080` via `ClusterIP`:

```yaml
spec:
  selector:
    app: go-server
  type: ClusterIP
  ports:
    - name: go-server-service
      port: 80        # porta exposta pelo Service dentro do cluster
      targetPort: 8080 # porta que o container realmente escuta
      protocol: TCP
```

### Tipos de Service

| Type           | Acesso                                          | Uso típico                                           |
| -------------- | ----------------------------------------------- | ---------------------------------------------------- |
| `ClusterIP`    | Somente dentro do cluster                       | Comunicação interna entre serviços                   |
| `NodePort`     | Via IP do nó + porta fixa (30000–32767)         | Acesso externo em dev/testes sem load balancer       |
| `LoadBalancer` | Via IP externo provisionado pelo cloud provider | Exposição em produção na AWS, GCP, Azure etc.        |
| `ExternalName` | Alias DNS para um serviço externo               | Integrar serviços externos ao DNS interno do cluster |

#### ClusterIP (padrão)

Cria um IP virtual acessível apenas de dentro do cluster. É o tipo mais simples e mais usado para comunicação entre microserviços. Não é acessível de fora do cluster.

```yaml
type: ClusterIP
ports:
  - port: 8080 # porta que o Service expõe internamente
    targetPort: 8080 # porta que o container escuta (padrão: igual a port)
```

#### NodePort

Abre uma porta em **todos os nós** do cluster e redireciona o tráfego para o Service. Acessível de fora com `<IP-do-nó>:<nodePort>`. Útil para testes locais com kind/minikube sem precisar de load balancer.

```yaml
type: NodePort
ports:
  - port: 8080
    targetPort: 8080
    nodePort: 30080 # porta no nó (omitir para o K8s escolher automaticamente)
```

#### LoadBalancer

Provisiona automaticamente um load balancer externo no cloud provider (AWS ELB, GCP LB etc.) e atribui um IP público. Em ambientes locais como kind, o IP externo fica em `<pending>` a menos que ferramentas como MetalLB sejam configuradas.

```yaml
type: LoadBalancer
ports:
  - port: 80
    targetPort: 8080
```

#### ExternalName

Não cria proxy nem endpoints — apenas mapeia o nome do Service para um registro CNAME externo. Usado para abstrair serviços externos (ex: banco de dados gerenciado) dentro do DNS do cluster.

```yaml
type: ExternalName
externalName: meu-banco.rds.amazonaws.com
```

### Comandos kubectl (Service)

```bash
# Criar/aplicar o Service no cluster
kubectl apply -f config/service.yaml

# Listar Services
kubectl get services
kubectl get svc

# Ver detalhes do Service (endpoints, selector, portas)
kubectl describe service go-server-service

# Acessar o Service via port-forward (útil com ClusterIP em dev)
# formato: <porta-local>:<port-do-service>
kubectl port-forward service/go-server-service 8080:80

# Iniciar proxy para a API do Kubernetes (porta padrão 8001)
kubectl proxy

# Ou escolher uma porta específica
kubectl proxy --port=8080

# Com o proxy ativo, acessar o Service pelo caminho da API:
# http://localhost:8080/api/v1/namespaces/default/services/go-server-service/proxy/

# Deletar o Service
kubectl delete service go-server-service
kubectl delete svc go-server-service
```

---

## configmap-env.yaml

Define um **ConfigMap** — recurso do Kubernetes para armazenar configurações não-sensíveis como pares chave-valor. Desacopla a configuração da imagem Docker: a mesma imagem pode rodar com valores diferentes em cada ambiente (dev, staging, prod) sem precisar ser reconstruída.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: go-server-env
data:
  NAME: "Emerson"
  AGE: "39"
```

### Como injetar o ConfigMap no Deployment

Há duas formas de passar as variáveis para o container:

**1. Todas as chaves de uma vez com `envFrom`** (usado neste projeto):

```yaml
containers:
  - name: go-server
    image: emersondp07/hello-go:v3
    envFrom:
      - configMapRef:
          name: go-server-env  # injeta NAME e AGE como env vars automaticamente
```

**2. Chave por chave com `configMapKeyRef`** (mais verboso, permite renomear a variável):

```yaml
containers:
  - name: go-server
    env:
      - name: NAME
        valueFrom:
          configMapKeyRef:
            name: go-server-env
            key: NAME
      - name: AGE
        valueFrom:
          configMapKeyRef:
            name: go-server-env
            key: AGE
```

> Use `envFrom` quando quiser importar todas as chaves de uma vez. Use `configMapKeyRef` quando precisar selecionar chaves específicas ou renomeá-las dentro do container.

### Comandos kubectl (ConfigMap)

```bash
# Criar/aplicar o ConfigMap
kubectl apply -f config/configmap-env.yaml

# Listar ConfigMaps
kubectl get configmaps
kubectl get cm

# Ver os dados do ConfigMap
kubectl describe configmap go-server-env

# Ver em formato yaml (mostra os valores)
kubectl get configmap go-server-env -o yaml

# Deletar o ConfigMap
kubectl delete configmap go-server-env
kubectl delete cm go-server-env
```

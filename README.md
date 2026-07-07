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

### resources (requests e limits)

Define quanto de CPU e memória o container pode usar. O Kubernetes usa esses valores para duas finalidades distintas:

```yaml
resources:
  requests:
    cpu: "0.3"    # 0.3 de 1 CPU (300 millicores)
    memory: 20Mi  # mínimo garantido para o container rodar
  limits:
    cpu: "0.3"    # teto máximo de CPU — não pode ultrapassar
    memory: 25Mi  # teto máximo de memória — se ultrapassar, o container é morto (OOMKilled)
```

- **`requests`**: quantidade que o K8s **reserva** no nó para o container. O scheduler usa esse valor para decidir em qual nó alocar o Pod — se nenhum nó tiver o suficiente disponível, o Pod fica `Pending`.
- **`limits`**: quantidade **máxima** que o container pode consumir. CPU acima do limite é throttled (desacelerado). Memória acima do limite causa `OOMKilled` e o container é reiniciado.

> Manter `requests` igual a `limits` (como neste caso com CPU) garante comportamento previsível e é necessário para que o HPA calcule o percentual de uso corretamente — ele divide o consumo atual pelo valor de `requests`.

### Probes: startupProbe, readinessProbe e livenessProbe

As três probes verificam a saúde do container via `GET /healthz`, mas cada uma tem um propósito e uma consequência diferente quando falha:

| Probe | Quando atua | O que faz ao falhar |
|---|---|---|
| `startupProbe` | Só durante a inicialização | Bloqueia as outras probes até passar — evita reiniciar um container que só está demorando para subir |
| `readinessProbe` | Continuamente, após o startup | Remove o Pod do Service (para de receber tráfego), sem reiniciar o container |
| `livenessProbe` | Continuamente, após o startup | Reinicia o container |

```yaml
startupProbe:
  httpGet:
    path: /healthz
    port: 8080
  periodSeconds: 3       # verifica a cada 3s
  failureThreshold: 30    # até 30 falhas (90s) antes de considerar startup travado

readinessProbe:
  httpGet:
    path: /healthz
    port: 8080
  periodSeconds: 3
  failureThreshold: 1    # 1 falha já remove o Pod do Service
  # initialDelaySeconds: 10

livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  periodSeconds: 5
  failureThreshold: 1    # 1 falha já reinicia o container
  timeoutSeconds: 1
  successThreshold: 1
  # initialDelaySeconds: 15
```

Enquanto o `startupProbe` não passar, o Kubernetes **não executa** `readinessProbe` nem `livenessProbe` — isso evita que um container que demora para inicializar seja reiniciado ou tirado de circulação por engano.

No `server.go`, o handler `/healthz` agora só responde `200` quando o tempo de execução está **entre 10 e 30 segundos**; fora desse intervalo retorna `500`:

```go
if durantion.Seconds() < 10 || durantion.Seconds() > 30 {
    w.WriteHeader(500) // ainda "subindo" (< 10s) ou "degradado" (> 30s)
} else {
    w.WriteHeader(200)
}
```

Isso simula o ciclo de vida completo:
1. **0–10s**: `/healthz` retorna `500` → `startupProbe` ainda está tentando (tem até 90s de tolerância, então não falha o Pod)
2. **10–30s**: `/healthz` retorna `200` → `startupProbe` passa, Pod fica `Ready` e começa a receber tráfego
3. **> 30s**: `/healthz` volta a retornar `500` → `readinessProbe` falha primeiro (tira o Pod do Service) e, em seguida, `livenessProbe` falha e reinicia o container — recomeçando o ciclo

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

## service-nodeport.yaml e service-loadbalancer.yaml

Duas variações do mesmo `go-server-service` visto acima, uma para cada `type` de Service exposto externamente. Servem como referência de estudo — **não aplique as duas junto com `service.yaml`**: como as três usam `metadata.name: go-server-service`, aplicar mais de uma sobrescreve a anterior (o último `kubectl apply` "ganha").

**`service-nodeport.yaml`** — expõe a porta em todos os nós do cluster:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: go-server-service
spec:
  selector:
    app: go-server
  type: NodePort
  ports:
    - name: go-server-service
      port: 80
      targetPort: 8080
      protocol: TCP
      nodePort: 30001
```

- **`nodePort: 30001`**: porta fixa aberta em **todos** os nós (control-plane e workers). Acessível via `<IP-do-nó>:30001`. Se omitido, o Kubernetes escolhe uma porta aleatória na faixa `30000–32767`.

**`service-loadbalancer.yaml`** — pede um load balancer externo ao provedor do cluster:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: go-server-service
spec:
  selector:
    app: go-server
  type: LoadBalancer
  ports:
    - name: go-server-service
      port: 80
      targetPort: 8080
      protocol: TCP
```

- Sem um cloud provider (ou algo como MetalLB) para provisionar o balanceador, o `EXTERNAL-IP` fica `<pending>` para sempre — é o que acontece no cluster local `kind` deste projeto (visto em `kubectl get svc`).

### Comandos kubectl (aplicar uma variante específica)

```bash
# Trocar o Service ativo para NodePort
kubectl apply -f config/service-nodeport.yaml

# Trocar o Service ativo para LoadBalancer
kubectl apply -f config/service-loadbalancer.yaml

# Conferir o type e as portas atuais
kubectl get svc go-server-service
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

---

## secret.yaml

Define um **Secret** — recurso do Kubernetes para armazenar dados sensíveis (senhas, tokens, chaves). Funciona de forma similar ao ConfigMap, mas os valores são armazenados em **Base64** e o acesso pode ser restrito via RBAC.

> Base64 **não é criptografia** — é apenas encoding. O Secret existe para separar dados sensíveis dos manifestos comuns e evitar que apareçam em logs ou no histórico do shell.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: go-server-secret
type: Opaque
data:
  USER: "RW1lcnNvbgo="      # "Emerson" em Base64
  PASSWORD: "MTAwOTE5OTYK"  # valor em Base64
```

O tipo `Opaque` é o genérico — usado para qualquer par chave-valor arbitrário.

### Como codificar/decodificar Base64

```bash
# Codificar um valor para colocar no Secret
echo -n "minha-senha" | base64

# Decodificar para conferir o valor real
echo "bWluaGEtc2VuaGE=" | base64 --decode
```

### Injetando o Secret no Deployment via `envFrom`

O Deployment usa `envFrom` com múltiplas fontes — ConfigMap e Secret são carregados juntos:

```yaml
envFrom:
  - configMapRef:
      name: go-server-env      # injeta NAME e AGE
  - secretRef:
      name: go-server-secret   # injeta USER e PASSWORD
```

O container recebe todas as chaves como variáveis de ambiente. Se houver chaves com o mesmo nome em fontes diferentes, a última declarada sobrescreve.

### Comandos kubectl (Secret)

```bash
# Criar/aplicar o Secret
kubectl apply -f config/secret.yaml

# Listar Secrets
kubectl get secrets

# Ver metadados (os valores ficam ocultos)
kubectl describe secret go-server-secret

# Ver os valores em Base64 (decodifique manualmente se precisar)
kubectl get secret go-server-secret -o yaml

# Deletar o Secret
kubectl delete secret go-server-secret
```

---

## configmap-family.yaml e volumeMounts

Além de injetar variáveis de ambiente, um ConfigMap pode ser montado como **arquivo dentro do container** via `volumeMounts`. Isso é útil quando a aplicação espera ler uma configuração de um caminho no filesystem.

O `configmap-family` armazena uma string com membros da família:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: configmap-family
data:
  members: "Emerson, Julianny, Edna"
```

No Deployment, esse ConfigMap é montado como o arquivo `/go/myfamily/family.txt`:

```yaml
containers:
  - name: go-server
    volumeMounts:
      - mountPath: "/go/myfamily"  # diretório criado dentro do container
        name: config
        readOnly: true             # container não pode modificar o arquivo

volumes:
  - name: config
    configMap:
      name: configmap-family
      items:
        - key: members         # chave do ConfigMap
          path: "family.txt"   # nome do arquivo gerado dentro do mountPath
```

O fluxo é: `volumes` define a fonte (o ConfigMap) → `volumeMounts` define onde montar dentro do container. O campo `items` seleciona quais chaves viram arquivos e com qual nome — sem ele, cada chave vira um arquivo separado com o próprio nome da chave.

O handler `/configmap` no `server.go` lê esse arquivo com `ioutil.ReadFile("/go/myfamily/family.txt")` e retorna o conteúdo na resposta HTTP.

### Comandos kubectl (configmap-family)

```bash
# Criar/aplicar o ConfigMap de família
kubectl apply -f config/configmap-family.yaml

# Confirmar que o arquivo foi montado no container
kubectl exec -it <nome-do-pod> -- cat /go/myfamily/family.txt

# Listar todos os ConfigMaps
kubectl get configmaps
```

---

## metrics-server.yaml

Instala o **metrics-server** — componente que coleta métricas de uso de CPU/memória de Pods e nós, expostas na API `metrics.k8s.io`. É um pré-requisito do `hpa.yaml` (seção seguinte): sem ele, o HPA não tem de onde ler o consumo de CPU e fica com status `<unknown>`.

Este arquivo é o manifesto oficial do projeto [kubernetes-sigs/metrics-server](https://github.com/kubernetes-sigs/metrics-server), praticamente sem alterações. Ele cria, no namespace `kube-system`:

| Recurso | Papel |
|---|---|
| `ServiceAccount` + `ClusterRole`/`ClusterRoleBinding` (x4) | RBAC: permite ao metrics-server ler métricas de `pods`/`nodes` e ser autenticado pela API do cluster |
| `Deployment` | O próprio metrics-server, que fala com o kubelet de cada nó para coletar as métricas |
| `Service` | Expõe o Deployment na porta `443` para a API do Kubernetes consultar |
| `APIService (v1beta1.metrics.k8s.io)` | Registra o endpoint `metrics.k8s.io` na API aggregation layer, é o que permite `kubectl top` e o HPA funcionarem |

A única flag ajustada para funcionar em clusters locais (como `kind`) é `--kubelet-insecure-tls` nos `args` do Deployment:

```yaml
args:
  - --cert-dir=/tmp
  - --secure-port=10250
  - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
  - --kubelet-use-node-status-port
  - --kubelet-insecure-tls   # ignora o certificado (auto-assinado) do kubelet
  - --metric-resolution=15s
```

> Em produção normalmente **não** se usa `--kubelet-insecure-tls` — em clusters gerenciados (EKS, GKE, AKS) o certificado do kubelet costuma ser válido e essa flag fica de fora.

### Comandos kubectl (metrics-server)

```bash
# Instalar o metrics-server
kubectl apply -f config/metrics-server.yaml

# Conferir que o Deployment ficou pronto (namespace kube-system)
kubectl get deployment metrics-server -n kube-system

# Testar se a API de métricas está respondendo
kubectl top nodes
kubectl top pods

# Remover o metrics-server
kubectl delete -f config/metrics-server.yaml
```

---

## hpa.yaml

Define um **HorizontalPodAutoscaler (HPA)** — recurso do Kubernetes que escala automaticamente o número de réplicas de um Deployment com base no consumo de CPU (ou outras métricas). Quando a carga aumenta, o HPA cria mais Pods; quando cai, remove o excesso.

```yaml
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    name: go-server
    kind: Deployment          # qual Deployment escalar
  minReplicas: 1              # mínimo de Pods, mesmo sem carga
  maxReplicas: 5              # máximo que pode escalar
  targetCPUUtilizationPercentage: 25  # escala quando CPU média passar de 25% do request
```

O HPA calcula o uso de CPU de todos os Pods do Deployment e compara com o `targetCPUUtilizationPercentage` aplicado sobre o `requests.cpu` definido no container. Se a média ultrapassar 25%, novos Pods são criados até o limite de 5.

> **Requisito**: o HPA depende do **metrics-server** instalado no cluster para coletar as métricas de CPU/memória dos Pods. Sem ele, o HPA fica com status `unknown` e não escala.

### Comandos kubectl (HPA)

```bash
# Criar/aplicar o HPA
kubectl apply -f config/hpa.yaml

# Listar HPAs e ver status atual
kubectl get hpa

# Ver detalhes: réplicas atuais, uso de CPU, eventos de escalonamento
kubectl describe hpa go-server-hpa

# Monitorar em tempo real (atualiza a cada 5s)
kubectl get hpa -w

# Deletar o HPA (o Deployment continua com as réplicas que estiver)
kubectl delete hpa go-server-hpa
```

---

## pv.yaml

Define um **PersistentVolume (PV)** — recurso de armazenamento no cluster com ciclo de vida independente de qualquer Pod. Diferente do volume `configMap` usado em `configmap-family.yaml` (que só existe enquanto referenciado), o PV é um objeto próprio do cluster: pode ser reaproveitado por Pods diferentes ao longo do tempo, mesmo que sejam recriados.

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: go-server-pv
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
    - ReadWriteMany
```

- **`capacity.storage`**: tamanho do volume (aqui, `1Gi`).
- **`accessModes`**: formas de montagem que o volume suporta:

| accessMode | Significado |
|---|---|
| `ReadWriteOnce` (RWO) | Montado para leitura/escrita por um único nó por vez |
| `ReadOnlyMany` (ROX) | Montado somente leitura por múltiplos nós simultaneamente |
| `ReadWriteMany` (RWX) | Montado para leitura/escrita por múltiplos nós simultaneamente |

> Um PV pode listar mais de um `accessMode` como suportado, mas cada montagem em uso usa apenas um deles por vez — quem escolhe é a **PersistentVolumeClaim (PVC)** que reivindica o volume.

> **Importante**: este manifesto não declara uma origem de armazenamento (`hostPath`, `nfs`, `csi` etc.), então o PV fica registrado na API mas sem um backend real de disco. Além disso, um PV sozinho não é montado em nenhum Pod — é necessário criar uma **PVC** que o reivindique e referenciar essa PVC no `volumes` do Deployment (o mesmo padrão já usado com `configMap` em `configmap-family.yaml`, trocando `configMap` por `persistentVolumeClaim`).

### Comandos kubectl (PersistentVolume)

```bash
# Criar/aplicar o PersistentVolume
kubectl apply -f config/pv.yaml

# Listar PersistentVolumes e ver status (Available, Bound, Released)
kubectl get pv

# Ver detalhes do PV (capacidade, access modes, claim vinculada)
kubectl describe pv go-server-pv

# Deletar o PersistentVolume
kubectl delete pv go-server-pv
```

---

## pvc.yaml

Define uma **PersistentVolumeClaim (PVC)** — o "pedido" de armazenamento feito pela aplicação. Enquanto o PV é o recurso de armazenamento oferecido pelo cluster, a PVC descreve o que a aplicação precisa (tamanho, modo de acesso), e o Kubernetes procura um PV compatível para vincular (**bind**).

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: go-server-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```

- **`accessModes`**: modo(s) de acesso exigido(s) — precisa estar entre os modos que o PV oferece.
- **`resources.requests.storage`**: tamanho mínimo exigido. O Kubernetes procura um PV com capacidade `>=` esse valor.

Essa PVC pede `ReadWriteOnce` e `1Gi`. O PV `go-server-pv` (seção acima) oferece `ReadWriteOnce` + `ReadWriteMany` e exatamente `1Gi` — compatíveis, então o bind acontece automaticamente (nenhum dos dois define `storageClassName`, então ambos usam a classe vazia `""` e podem casar).

Para o Pod realmente usar esse armazenamento, falta referenciar a PVC no `volumes` do Deployment (mesmo padrão já usado com `configMap`, trocando por `persistentVolumeClaim`):

```yaml
volumes:
  - name: data
    persistentVolumeClaim:
      claimName: go-server-pvc
```

> Esse trecho ainda não está no `deployment.yaml` deste projeto — a PVC pode ficar `Bound` ao PV, mas nenhum Pod está montando o volume ainda.

### Comandos kubectl (PersistentVolumeClaim)

```bash
# Criar/aplicar a PVC
kubectl apply -f config/pvc.yaml

# Listar PVCs e ver status (Pending, Bound)
kubectl get pvc

# Ver detalhes (qual PV foi vinculado, capacidade, access modes)
kubectl describe pvc go-server-pvc

# Conferir que o PV correspondente mudou de Available para Bound
kubectl get pv

# Deletar a PVC
kubectl delete pvc go-server-pvc
```

---

## statefulset.yaml

Define um **StatefulSet** — recurso para aplicações que precisam de identidade estável e armazenamento persistente por réplica, como bancos de dados. Diferente do Deployment (onde os Pods são intercambiáveis entre si), cada Pod de um StatefulSet tem:

- **Nome previsível e fixo**: `mysql-0`, `mysql-1`, `mysql-2` (índice sequencial, não hash aleatório).
- **Ordem garantida** de criação, atualização e remoção: `mysql-0` sempre sobe antes de `mysql-1`, e é removido por último.
- **Armazenamento próprio por Pod**: se configurado com `volumeClaimTemplates`, cada réplica recebe sua própria PVC, que persiste mesmo se o Pod for recriado (ele volta com o mesmo disco).

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mysql
spec:
  serviceName: mysql-h
  replicas: 3
  selector:
    matchLabels:
      app: mysql
  template:
    metadata:
      labels:
        app: mysql
    spec:
      containers:
        - name: mysql
          image: mysql
          env:
            - name: MYSQL_ROOT_PASSWORD
              value: root
```

- **`serviceName`**: nome do Service **headless** (`clusterIP: None`) que gerencia a rede do StatefulSet. É ele quem dá a cada Pod um DNS estável: `mysql-0.mysql-h`, `mysql-1.mysql-h`, `mysql-2.mysql-h`.
- **`replicas: 3`**: três Pods MySQL, criados e escalados em ordem (`mysql-0` → `mysql-1` → `mysql-2`).
- Sem `volumeClaimTemplates` neste manifesto, os três Pods usam apenas o filesystem efêmero do container — cada um perde os dados do banco se for recriado. Persistência real por Pod exigiria algo como:

```yaml
volumeClaimTemplates:
  - metadata:
      name: mysql-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 1Gi
```

> Esse padrão gera automaticamente uma PVC por Pod (`mysql-data-mysql-0`, `mysql-data-mysql-1`, `mysql-data-mysql-2`), diferente da PVC única compartilhada usada em `pvc.yaml`.

> O Service headless `mysql-h` referenciado em `serviceName` é definido em [mysql-service-h.yaml](#mysql-service-hyaml), logo abaixo.

### Deployment vs. StatefulSet

| | Deployment | StatefulSet |
|---|---|---|
| Nome dos Pods | Hash aleatório (`go-server-5446c7d9c5-tw564`) | Índice fixo (`mysql-0`, `mysql-1`, ...) |
| Ordem de criação/remoção | Paralela, sem garantia | Sequencial (0, 1, 2, ...) |
| Armazenamento | Compartilhado ou nenhum | Uma PVC própria por Pod (via `volumeClaimTemplates`) |
| DNS | Só via Service comum | DNS estável por Pod via Service headless |
| Uso típico | Apps stateless (APIs, web servers) | Bancos de dados, filas, sistemas com estado |

### Comandos kubectl (StatefulSet)

```bash
# Criar/aplicar o StatefulSet
kubectl apply -f config/statefulset.yaml

# Listar StatefulSets
kubectl get statefulsets
kubectl get sts

# Ver detalhes (ordem de criação, eventos, réplicas prontas)
kubectl describe statefulset mysql

# Ver os Pods sendo criados em ordem (mysql-0, depois mysql-1, depois mysql-2)
kubectl get pods -l app=mysql -w

# Escalar o número de réplicas
kubectl scale statefulset mysql --replicas=5

# Deletar o StatefulSet (por padrão não deleta as PVCs geradas por volumeClaimTemplates)
kubectl delete statefulset mysql
kubectl delete sts mysql
```

---

## mysql-service-h.yaml

Define o Service **headless** que o StatefulSet `mysql` usa para dar identidade de rede estável a cada réplica.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: mysql-h
spec:
  selector:
    app: mysql
  ports:
    - port: 3306
  clusterIP: None
```

- **`clusterIP: None`**: é isso que torna o Service "headless". Um Service normal cria um IP virtual único e faz load balancing entre os Pods que casam com o `selector`; um Service headless não cria IP nenhum — o CoreDNS passa a resolver o nome do Service diretamente para os IPs de **cada Pod individualmente**.
- **`selector: app: mysql`**: casa com os Pods criados pelo `statefulset.yaml`.
- **`port: 3306`**: porta padrão do MySQL, a mesma exposta pelo container `image: mysql` do StatefulSet.

Combinando esse Service com `serviceName: mysql-h` no StatefulSet, cada réplica ganha um registro DNS próprio e previsível, no formato `<nome-do-pod>.<serviceName>`:

- `mysql-0.mysql-h`
- `mysql-1.mysql-h`
- `mysql-2.mysql-h`

Isso é o que diferencia StatefulSet de Deployment na prática: com Deployment + Service comum, só é possível falar com "algum Pod" (round-robin, sem controle de qual). Com StatefulSet + Service headless, é possível endereçar um Pod específico — essencial para MySQL, onde tipicamente um nó é o primary (aceita escrita) e os demais são replicas (só leitura), e a aplicação precisa diferenciar `mysql-0` dos outros.

### Comandos kubectl (Service headless)

```bash
# Criar/aplicar o Service headless
kubectl apply -f config/mysql-service-h.yaml

# Ver que não há CLUSTER-IP (aparece "None")
kubectl get service mysql-h

# Ver os endpoints — um IP por Pod, ao contrário de um Service normal
kubectl get endpoints mysql-h

# Resolver o DNS de um Pod específico de dentro do cluster (a partir de outro pod)
kubectl run -it --rm dns-test --image=busybox -- nslookup mysql-0.mysql-h

# Deletar o Service headless
kubectl delete service mysql-h
```

---

## ingress.yaml

Define um **Ingress** — recurso que roteia tráfego HTTP/HTTPS externo para Services internos com base em regras de host/path, evitando expor um `LoadBalancer` ou `NodePort` por serviço. Quem interpreta essas regras é o **Ingress Controller** (o `ingress-nginx` instalado via Helm neste projeto).

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-host
  annotations:
    kubernetes.io/ingress.class: "nginx"
    cert-manager.io/cluster-issuer: "letsencrypt"
    ingress.kubernetes.io/force-ssl-redirect: "true"
spec:
  rules:
    - host: "example.com"
      http:
        paths:
          - pathType: Prefix
            path: "/"
            backend:
              service:
                name: go-server-service
                port:
                  number: 80
  tls:
    - hosts:
        - "example.com"
      secretName: letsencrypt-tls
```

- **`rules[].host`**: domínio que aciona essa regra (`example.com`, apenas ilustrativo — precisaria apontar de fato para o IP do Ingress Controller via DNS).
- **`rules[].http.paths`**: cada path define para qual Service/porta o tráfego é roteado (`pathType: Prefix` casa `/` e qualquer coisa abaixo).
- **`tls`**: habilita HTTPS para os hosts listados, usando o certificado guardado no Secret `letsencrypt-tls` — Secret esse que o `cert-manager` cria automaticamente a partir do `ClusterIssuer` (próxima seção).
- **`cert-manager.io/cluster-issuer`**: annotation lida pelo `cert-manager` (não pelo ingress-nginx) — instrui a gerar/renovar o certificado TLS usando o `ClusterIssuer` chamado `letsencrypt`.

> **Nota**: `kubernetes.io/ingress.class` está deprecated em favor do campo `spec.ingressClassName: nginx`, e `ingress.kubernetes.io/force-ssl-redirect` é a annotation legada do ingress-nginx — a atual é `nginx.ingress.kubernetes.io/force-ssl-redirect`. Ambas tendem a ser ignoradas silenciosamente pelas versões recentes do controller, em vez de dar erro (não impedem o `apply`, mas valem a pena atualizar).

### Comandos kubectl (Ingress)

```bash
# Criar/aplicar o Ingress
kubectl apply -f config/ingress.yaml

# Listar Ingresses e ver o endereço atribuído
kubectl get ingress

# Ver detalhes (regras, backend, eventos do controller)
kubectl describe ingress ingress-host

# Testar localmente sem DNS real, resolvendo o host manualmente
curl -H "Host: example.com" http://localhost/

# Deletar o Ingress
kubectl delete ingress ingress-host
```

---

## cluster-issuer.yaml

Define um **ClusterIssuer** — recurso do `cert-manager` que descreve *como* emitir certificados TLS (neste caso, via Let's Encrypt/ACME). É "Cluster" porque pode ser referenciado por Ingresses de qualquer namespace, diferente de um `Issuer` comum (namespaced).

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt
  namespace: cert-manager
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: emerson.dantaspereira@hotmail.com
    privateKeySecretRef:
      name: letsencrypt-tls
    solvers:
      - http01:
          ingress:
            class: nginx
```

- **`spec.acme.server`**: endpoint do diretório ACME do Let's Encrypt — este é o ambiente de **produção** (rate limits baixos; para testes, o normal é usar primeiro `https://acme-staging-v02.api.letsencrypt.org/directory`).
- **`privateKeySecretRef.name`**: Secret onde o cert-manager guarda a chave privada da conta ACME (não é o Secret do certificado do site, esse é o `letsencrypt-tls` referenciado no `tls.secretName` do `ingress.yaml`).
- **`solvers[].http01.ingress.class`**: resolve o desafio `HTTP-01` criando temporariamente um path no Ingress Controller `nginx` para o Let's Encrypt validar posse do domínio.

> **Nota**: `ClusterIssuer` é um recurso **cluster-scoped** (não pertence a namespace algum); o campo `metadata.namespace: cert-manager` é ignorado pela API, mas deixa a leitura do manifesto confusa.

### Comandos kubectl (ClusterIssuer)

```bash
# Criar/aplicar o ClusterIssuer
kubectl apply -f config/cluster-issuer.yaml

# Listar ClusterIssuers e ver se está Ready
kubectl get clusterissuer

# Ver detalhes (condições, erros de emissão)
kubectl describe clusterissuer letsencrypt

# Acompanhar o Certificate gerado a partir do Ingress (se o cert-manager estiver instalado)
kubectl get certificate -A

# Deletar o ClusterIssuer
kubectl delete clusterissuer letsencrypt
```

---

## Namespaces (config/namespaces/)

Um **Namespace** cria uma divisão lógica dentro do mesmo cluster físico — como "pastas" que isolam nomes de recursos entre si. Dois recursos podem ter o mesmo `metadata.name` desde que estejam em namespaces diferentes (é por isso que o `ingress-nginx-controller` e o `go-server-service` puderam conviver sem conflito quando estavam em namespaces distintos, visto mais cedo nesta conversa). Recursos **cluster-scoped** (como `PersistentVolume` e `ClusterIssuer`) não pertencem a namespace nenhum — só recursos namespaced (`Pod`, `Service`, `Deployment`, `ConfigMap` etc.) são afetados.

Este projeto tem um exemplo em `config/namespaces/deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: server
spec:
  selector:
    matchLabels:
      app: server
  template:
    metadata:
      labels:
        app: server
    spec:
      containers:
        - name: server
          image: <Image>
          resources:
            limits:
              memory: "128Mi"
              cpu: "500m"
          ports:
            - containerPort: 3000
```

> Esse manifesto não define `metadata.namespace` — o namespace de destino é decidido no momento do `apply` (via `-n`) ou pelo namespace padrão do contexto atual (ver seção **Context** abaixo). O campo `image: <Image>` também é só um placeholder — precisa ser substituído por uma imagem real antes de aplicar.

### Criando e usando Namespaces

**Forma imperativa** (rápida, sem YAML):

```bash
kubectl create namespace estudos
```

**Forma declarativa** (versionável, mesmo padrão dos outros arquivos deste repo):

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: estudos
```

Depois de criado, um recurso pode ser direcionado ao namespace de duas formas:

```yaml
# 1. Declarando no próprio manifesto
metadata:
  name: server
  namespace: estudos
```

```bash
# 2. Passando -n/--namespace no apply (sobrescreve o que estiver no manifesto)
kubectl apply -f config/namespaces/deployment.yaml -n estudos
```

### Comandos kubectl (Namespace)

```bash
# Criar o namespace
kubectl create namespace estudos

# Listar namespaces existentes
kubectl get namespaces
kubectl get ns

# Aplicar um recurso dentro de um namespace específico
kubectl apply -f config/namespaces/deployment.yaml -n estudos

# Listar recursos de um namespace específico
kubectl get pods -n estudos

# Listar recursos de TODOS os namespaces de uma vez
kubectl get pods -A
kubectl get pods --all-namespaces

# Ver detalhes de um namespace
kubectl describe namespace estudos

# Deletar o namespace (remove TODOS os recursos dentro dele também)
kubectl delete namespace estudos
```

---

## Context (kubeconfig)

Um **context** no `kubeconfig` (`~/.kube/config`) é a combinação de **cluster + usuário + namespace padrão**. É o que permite alternar entre clusters diferentes (ex: `kind` local vs. um cluster de produção) ou entre namespaces diferentes do mesmo cluster, sem precisar passar `--context`/`--namespace` em todo comando.

Ao criar um cluster com `kind create cluster --config config/kind.yaml`, o kind já cria e ativa automaticamente um context chamado `kind-<nome-do-cluster>`.

### Comandos kubectl (context)

```bash
# Ver o context ativo no momento
kubectl config current-context

# Listar todos os contexts disponíveis no kubeconfig
kubectl config get-contexts

# Trocar de context (ex: mudar de cluster)
kubectl config use-context kind-fullcycle

# Mudar o namespace padrão do context atual (evita repetir -n em todo comando)
kubectl config set-context --current --namespace=estudos

# Conferir o namespace padrão configurado no context atual
kubectl config view --minify | grep namespace

# Ver o kubeconfig completo (clusters, usuários e contexts)
kubectl config view
```

> Depois de rodar `set-context --current --namespace=estudos`, comandos como `kubectl get pods` passam a listar o namespace `estudos` por padrão, sem precisar de `-n estudos` a cada vez — útil quando se está trabalhando em um namespace por um bom tempo.

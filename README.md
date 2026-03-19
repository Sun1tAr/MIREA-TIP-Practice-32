# Практическое занятие №16 (32). Публикация приложения в Kubernetes (минимальный манифест)

## Выполнил: Туев Д. ЭФМО-01-25

## Содержание

1. [Описание проекта](#описание-проекта)
2. [Выбор и настройка Kubernetes стенда](#выбор-и-настройка-kubernetes-стенда)
3. [Подготовка Docker образов](#подготовка-docker-образов)
4. [Структура манифестов](#структура-манифестов)
5. [ConfigMap: конфигурация приложения](#configmap-конфигурация-приложения)
6. [Secrets: хранение чувствительных данных](#secrets-хранение-чувствительных-данных)
7. [Базы данных: PostgreSQL, Redis, RabbitMQ](#базы-данных-postgresql-redis-rabbitmq)
8. [Deployment сервисов: Auth, Tasks, Worker, GraphQL](#deployment-сервисов-auth-tasks-worker-graphql)
9. [Service: внутренний доступ к подам](#service-внутренний-доступ-к-подам)
10. [Ingress: доступ извне](#ingress-доступ-извне)
11. [Применение манифестов и проверка](#применение-манифестов-и-проверка)
12. [Масштабирование](#масштабирование)
13. [Скриншоты выполнения](#скриншоты-выполнения)
14. [Выводы](#выводы)
15. [Контрольные вопросы](#контрольные-вопросы)

---

## Описание проекта

В рамках практического занятия №16 (32) выполнена публикация полного стека сервисов (auth, tasks, worker, graphql) в Kubernetes с использованием минимальных манифестов. Проект развёрнут на отдельном сервере с использованием k3s — легковесного сертифицированного Kubernetes.

**Архитектура решения:**

- **Kubernetes стенд:** k3s на отдельном сервере
- **Реестр образов:** GitHub Container Registry (GHCR)
- **Базы данных:** PostgreSQL, Redis, RabbitMQ (StatefulSets с PersistentVolume)
- **Сервисы:** Auth, Tasks, Worker, GraphQL (Deployments с репликами)
- **Доступ извне:** Ingress Controller (nginx) + Ingress ресурс с правилами маршрутизации
- **Конфигурация:** ConfigMap для несекретных параметров, Secrets для паролей
- **Мониторинг:** readiness/liveness пробы для всех сервисов

**Развёрнутые сервисы и их эндпоинты:**

| Сервис | Внутренний порт | Внешний URL (через Ingress) |
|--------|-----------------|------------------------------|
| **Auth** | 8081 (HTTP), 50051 (gRPC) | `http://denchik23761.fvds.ru:30508/auth-service/v1/auth/login` |
| **Tasks** | 8082 | `http://denchik23761.fvds.ru:30508/tasks-service/v1/tasks` |
| **Worker** | - (нет HTTP) | - |
| **GraphQL** | 8090 | `http://denchik23761.fvds.ru:30508/graphql-service/query` |
| **Health** | - | `http://denchik23761.fvds.ru:30508/health-checks/health` |
| **Metrics** | - | `http://denchik23761.fvds.ru:30508/health-checks/metrics` |

---

## Выбор и настройка Kubernetes стенда

Для выполнения работы был выбран **k3s** — сертифицированный Kubernetes от Rancher, идеально подходящий для небольших серверов и edge-устройств.

### Установка k3s на сервер

```bash
# Подключение к серверу
ssh root@188.120.236.112

# Установка k3s одной командой
curl -sfL https://get.k3s.io | sh -

# Проверка установки
kubectl get nodes
```

Вывод команды:
```
NAME                   STATUS   ROLES                  AGE   VERSION
denchik23761.fvds.ru   Ready    control-plane,master   1d    v1.30.5+k3s1
```

### Настройка доступа kubectl с локальной машины

Для управления кластером с локальной машины был скопирован конфиг:

```powershell
# Копирование конфига с сервера
scp root@188.120.236.112:/etc/rancher/k3s/k3s.yaml ~/.kube/config

# Замена localhost на внешний IP в конфиге
notepad ~/.kube/config
```

В файле конфига строка `server: https://127.0.0.1:6443` была заменена на `server: https://188.120.236.112:6443`.

### Открытие порта 6443 в firewall

Для доступа к API сервера извне потребовалось открыть порт 6443:

```bash
# На сервере
sudo ufw allow 6443/tcp
sudo ufw reload
```

После этого `kubectl` с локальной машины успешно подключился к кластеру:

```powershell
kubectl get nodes
```

---

## Подготовка Docker образов

Все сервисы были собраны в Docker образы и загружены в GitHub Container Registry (GHCR).

### Сборка образов

```powershell
# Переход в корень проекта
cd C:\_Projects\ExamplesCluster\MIREA-TIP-Practice-32

# Сборка образов с тегами
docker build -t ghcr.io/sun1tar/mirea-tip-practice-32/auth:health-v1 ./tech-ip-sem2/services/auth
docker build -t ghcr.io/sun1tar/mirea-tip-practice-32/tasks:health-v1 ./tech-ip-sem2/services/tasks
docker build -t ghcr.io/sun1tar/mirea-tip-practice-32/worker:health-v1 ./tech-ip-sem2/services/worker
docker build -t ghcr.io/sun1tar/mirea-tip-practice-32/graphql:health-v1 ./tech-ip-sem2/services/graphql
```

### Публикация в GHCR

```powershell
# Логин в GHCR (используется GitHub токен)
echo your_git_access_token | docker login ghcr.io -u sun1tar --password-stdin

# Публикация образов
docker push ghcr.io/sun1tar/mirea-tip-practice-32/auth:health-v1
docker push ghcr.io/sun1tar/mirea-tip-practice-32/tasks:health-v1
docker push ghcr.io/sun1tar/mirea-tip-practice-32/worker:health-v1
docker push ghcr.io/sun1tar/mirea-tip-practice-32/graphql:health-v1
```

### Создание секрета для доступа к образам в кластере

```bash
# На сервере
kubectl create namespace tip-practice-32
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=sun1tar \
  --docker-password="your_git_access_token" \
  --namespace=tip-practice-32
```

Этот секрет был добавлен во все Deployment'ы для возможности скачивания приватных образов.

---

## Структура манифестов

Все манифесты организованы в папке `deploy/k8s`:

```
deploy/k8s/
├── namespace.yaml
├── secrets.yaml
├── configmap.yaml
├── postgres.yaml
├── redis.yaml
├── rabbitmq.yaml
├── auth.yaml
├── tasks.yaml
├── worker.yaml
├── graphql.yaml
└── nginx.yaml (Ingress)
```

---

## ConfigMap: конфигурация приложения

ConfigMap хранит все несекретные параметры конфигурации, которые могут быть общими для нескольких сервисов.

### 📄 `deploy/k8s/configmap.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: tip-practice-32
data:
  # Database
  DB_HOST: "postgres-service"
  DB_PORT: "5432"
  DB_USER: "tasks_user"
  DB_NAME: "tasks_db"
  DB_DRIVER: "postgres"
  
  # Redis
  REDIS_ADDR: "redis-service:6379"
  REDIS_DB: "0"
  CACHE_TTL_SECONDS: "120"
  CACHE_TTL_JITTER_SECONDS: "30"
  
  # RabbitMQ очереди
  MAIN_QUEUE: "task_jobs"
  RETRY_QUEUE: "task_jobs_retry"
  DLQ: "task_jobs_dlq"
  RABBITMQ_PREFETCH: "1"
  MAX_ATTEMPTS: "3"
  RETRY_DELAY_SECONDS: "10"
  
  # Сервисы
  AUTH_HTTP_PORT: "8081"
  AUTH_GRPC_PORT: "50051"
  TASKS_PORT: "8082"
  GRAPHQL_PORT: "8090"
  LOG_LEVEL: "info"
  
  # Для tasks
  RABBITMQ_QUEUE: "task_events"
```

**Назначение параметров:**

| Параметр | Описание |
|----------|----------|
| `DB_HOST` | Имя сервиса PostgreSQL в кластере |
| `DB_USER`/`DB_NAME` | Учётные данные для подключения к БД |
| `REDIS_ADDR` | Адрес Redis сервиса |
| `MAIN_QUEUE`/`RETRY_QUEUE`/`DLQ` | Очереди RabbitMQ для обработки задач с retry |
| `MAX_ATTEMPTS` | Максимальное количество попыток обработки задачи |
| `RETRY_DELAY_SECONDS` | Задержка перед повторной попыткой (через TTL) |

---

## Secrets: хранение чувствительных данных

Secrets используются для хранения паролей и других конфиденциальных данных.

### 📄 `deploy/k8s/secrets.yaml`

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: app-secrets
  namespace: tip-practice-32
type: Opaque
stringData:
  DB_PASSWORD: "tasks_pass"
  REDIS_PASSWORD: ""
  RABBITMQ_URL: "amqp://guest:guest@rabbitmq-service:5672/"
```

Для доступа к образам из приватного реестра также был создан отдельный секрет:

```bash
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=sun1tar \
  --docker-password="<токен>" \
  --namespace=tip-practice-32
```

---

## Базы данных: PostgreSQL, Redis, RabbitMQ

Для каждого компонента данных использованы StatefulSet с PersistentVolume для сохранения данных при перезапусках.

### PostgreSQL (StatefulSet)

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  namespace: tip-practice-32
spec:
  serviceName: postgres-service
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:15-alpine
        ports:
        - containerPort: 5432
        env:
        - name: POSTGRES_USER
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: DB_USER
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: app-secrets
              key: DB_PASSWORD
        - name: POSTGRES_DB
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: DB_NAME
        volumeMounts:
        - name: data
          mountPath: /var/lib/postgresql/data
        - name: init-scripts
          mountPath: /docker-entrypoint-initdb.d
        readinessProbe:
          exec:
            command: ["pg_isready", "-U", "tasks_user"]
          initialDelaySeconds: 5
          periodSeconds: 5
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 1Gi
```

Аналогично были настроены **Redis** (Deployment с PVC) и **RabbitMQ** (StatefulSet).

### Особенности RabbitMQ

Для RabbitMQ потребовалась настройка readiness/liveness probes с увеличенным таймаутом, так как стандартный `rabbitmq-diagnostics ping` иногда не успевал ответить за 1 секунду:

```yaml
readinessProbe:
  exec:
    command: ["rabbitmq-diagnostics", "ping"]
  initialDelaySeconds: 20
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3
```

После этой настройки RabbitMQ стабильно работал.

---

## Deployment сервисов: Auth, Tasks, Worker, GraphQL

### Auth Service

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth
  namespace: tip-practice-32
spec:
  replicas: 2
  selector:
    matchLabels:
      app: auth
  template:
    metadata:
      labels:
        app: auth
    spec:
      imagePullSecrets:
      - name: ghcr-secret
      containers:
      - name: auth
        image: ghcr.io/sun1tar/mirea-tip-practice-32/auth:health-v1
        ports:
        - containerPort: 8081
          name: http
        - containerPort: 50051
          name: grpc
        env:
        - name: AUTH_HTTP_PORT
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: AUTH_HTTP_PORT
        - name: AUTH_GRPC_PORT
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: AUTH_GRPC_PORT
        - name: LOG_LEVEL
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: LOG_LEVEL
        readinessProbe:
          httpGet:
            path: /health
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 2
        livenessProbe:
          httpGet:
            path: /health
            port: 8081
          initialDelaySeconds: 15
          periodSeconds: 10
          timeoutSeconds: 2
        resources:
          requests:
            memory: "64Mi"
            cpu: "50m"
          limits:
            memory: "128Mi"
            cpu: "200m"
```

### Tasks Service

Tasks сервис имеет больше переменных окружения, включая подключение к БД, Redis и RabbitMQ:

```yaml
env:
- name: TASKS_PORT
  valueFrom:
    configMapKeyRef:
      name: app-config
      key: TASKS_PORT
- name: AUTH_GRPC_ADDR
  value: "auth-service:50051"
- name: DB_HOST
  valueFrom:
    configMapKeyRef:
      name: app-config
      key: DB_HOST
# ... и другие переменные
```

### Worker Service

Worker не имеет HTTP портов, поэтому для него не настроены HTTP пробы, только ресурсы:

```yaml
containers:
- name: worker
  image: ghcr.io/sun1tar/mirea-tip-practice-32/worker:health-v1
  env:
  - name: RABBITMQ_URL
    valueFrom:
      secretKeyRef:
        name: app-secrets
        key: RABBITMQ_URL
  - name: MAIN_QUEUE
    valueFrom:
      configMapKeyRef:
        name: app-config
        key: MAIN_QUEUE
  # ... остальные переменные
```

### GraphQL Service

Для GraphQL были добавлены хардкодные health handlers в `main.go`:

```go
// Health checks для Kubernetes
mux.HandleFunc("GET /health", healthHandler("graphql"))
mux.HandleFunc("GET /ready", readyHandler("graphql"))
```

---

## Service: внутренний доступ к подам

Для каждого сервиса создан Service типа ClusterIP для внутренней маршрутизации.

### Пример для Auth

```yaml
apiVersion: v1
kind: Service
metadata:
  name: auth-service
  namespace: tip-practice-32
spec:
  selector:
    app: auth
  ports:
    - name: http
      port: 8081
      targetPort: 8081
    - name: grpc
      port: 50051
      targetPort: 50051
```

Благодаря этому, другие сервисы могут обращаться к Auth по имени `auth-service:8081`.

---

## Ingress: доступ извне

Для доступа к сервисам из внешней сети использован Ingress Controller (nginx) и ресурс Ingress.

### 📄 `deploy/k8s/nginx.yaml`

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: tip-practice-32-ingress
  namespace: tip-practice-32
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /$2
    nginx.ingress.kubernetes.io/use-regex: "true"
spec:
  ingressClassName: nginx
  rules:
    - host: denchik23761.fvds.ru
      http:
        paths:
          # Auth endpoints
          - path: /MIREA/TIP-Practice/32/auth-service(/|$)(.*)
            pathType: Prefix
            backend:
              service:
                name: auth-service
                port:
                  number: 8081

          # Tasks API endpoints
          - path: /MIREA/TIP-Practice/32/tasks-service(/|$)(.*)
            pathType: Prefix
            backend:
              service:
                name: tasks-service
                port:
                  number: 8082

          # GraphQL endpoints
          - path: /MIREA/TIP-Practice/32/graphql-service(/|$)(.*)
            pathType: Prefix
            backend:
              service:
                name: graphql-service
                port:
                  number: 8090

          # Health endpoints для всех сервисов
          - path: /MIREA/TIP-Practice/32/health-checks(/|$)(.*)
            pathType: Prefix
            backend:
              service:
                name: tasks-service
                port:
                  number: 8082
```

### Особенности настройки Ingress

1. **Аннотация `rewrite-target: /$2`** — позволяет перенаправлять запросы с сохранением пути после префикса
2. **Использование регулярных выражений** — для гибкой маршрутизации
3. **Разделение по сервисам** — каждый сервис имеет свой префикс пути (`auth-service`, `tasks-service`, `graphql-service`, `health-checks`)

### Проблемы и их решение

В процессе настройки возникла проблема с портом: Ingress Controller использовал NodePort 30508 вместо стандартного 80. Это было обнаружено командой:

```powershell
kubectl get svc -n ingress-nginx
NAME                                 TYPE           CLUSTER-IP     EXTERNAL-IP   PORT(S)                      AGE
ingress-nginx-controller             LoadBalancer   10.43.83.164   <pending>     80:30508/TCP,443:30392/TCP   19h
```

В итоге все внешние запросы идут через порт **30508**:

```
http://denchik23761.fvds.ru:30508/MIREA/TIP-Practice/32/auth-service/v1/auth/login
```

---

## Применение манифестов и проверка

### Применение всех манифестов

```powershell
# Базовые ресурсы
kubectl apply -f deploy/k8s/namespace.yaml
kubectl apply -f deploy/k8s/secrets.yaml
kubectl apply -f deploy/k8s/configmap.yaml

# Базы данных
kubectl apply -f deploy/k8s/postgres.yaml
kubectl apply -f deploy/k8s/redis.yaml
kubectl apply -f deploy/k8s/rabbitmq.yaml

# Сервисы
kubectl apply -f deploy/k8s/auth.yaml
kubectl apply -f deploy/k8s/tasks.yaml
kubectl apply -f deploy/k8s/worker.yaml
kubectl apply -f deploy/k8s/graphql.yaml

# Ingress
kubectl apply -f deploy/k8s/nginx.yaml
```

### Проверка состояния подов

```powershell
kubectl get pods -n tip-practice-32
```

Результат после успешного запуска:

```
NAME                       READY   STATUS    RESTARTS   AGE
auth-bd8bb5bb-bpbcb        1/1     Running   0          45m
auth-bd8bb5bb-lx284        1/1     Running   0          45m
graphql-7b49c54c76-9thpb   1/1     Running   0          51m
postgres-0                 1/1     Running   0          134m
rabbitmq-0                 1/1     Running   0          106m
redis-87985d464-lcqf6      1/1     Running   0          134m
tasks-56dcdc5d8d-4ngg9     1/1     Running   0          45m
tasks-56dcdc5d8d-xw9xn     1/1     Running   0          45m
worker-5dcdff555d-2zgd7    1/1     Running   0          14m
```

### Проверка endpoints

```powershell
kubectl get endpoints -n tip-practice-32
```

```
NAME               ENDPOINTS                                                       AGE
auth-service       10.42.0.29:50051,10.42.0.30:50051,10.42.0.29:8081 + 1 more...   109m
graphql-service    10.42.0.32:8090                                                 108m
postgres-service   10.42.0.15:5432                                                 137m
rabbitmq-service   10.42.0.18:5672,10.42.0.18:15672                                137m
redis-service      10.42.0.16:6379                                                 137m
tasks-service      10.42.0.28:8082,10.42.0.31:8082                                 108m
```

### Проверка доступа через Ingress

```bash
# С сервера
curl -H "Host: denchik23761.fvds.ru" http://localhost:30508/MIREA/TIP-Practice/32/health-checks/health

# С локальной машины (через Postman)
GET http://denchik23761.fvds.ru:30508/MIREA/TIP-Practice/32/health-checks/health
```

### Port-forward для отладки

При необходимости можно использовать port-forward для прямого доступа к сервисам в обход Ingress:

```powershell
kubectl port-forward -n tip-practice-32 svc/tasks-service 8082:8082
```

---

## Масштабирование

Для демонстрации возможностей масштабирования было увеличено количество реплик tasks сервиса:

```powershell
kubectl scale deployment tasks -n tip-practice-32 --replicas=3
```

Результат:

```powershell
kubectl get pods -n tip-practice-32 | findstr tasks
tasks-56dcdc5d8d-4ngg9     1/1     Running   0          47m
tasks-56dcdc5d8d-xw9xn     1/1     Running   0          47m
tasks-56dcdc5d8d-xxxxx     1/1     Running   0          10s
```

Балансировка нагрузки между репликами автоматически обеспечивается Service и kube-proxy.

---

## Скриншоты выполнения

### 1. Установка k3s на сервер

![img.png](about/img.png)

### 2. Проверка доступности кластера с локальной машины

![img_1.png](about/img_1.png)

### 3. Сборка и публикация Docker образов

![img_2.png](about/img_2.png)

### 4. Создание namespace и секретов

![img_3.png](about/img_3.png)

### 5. Применение манифестов (на примере nginx)

![img_4.png](about/img_4.png)

### 6. Состояние подов после запуска

![img_5.png](about/img_5.png)

### 7. Проверка endpoints

![img_6.png](about/img_6.png)

### 8. Ingress и сервисы

![img_7.png](about/img_7.png)

### 9. Тестирование через Postman (логин)

![img_8.png](about/img_8.png)

### 10. Тестирование через Postman (создание задачи)

![img_9.png](about/img_9.png)

### 11. Тестирование GraphQL

![img_10.png](about/img_10.png)

### 12. Health checks

![img_11.png](about/img_11.png)



---

## Выводы

В ходе выполнения практического занятия №16 были достигнуты следующие результаты:

### Реализованная инфраструктура

1. **Kubernetes кластер** развёрнут на отдельном сервере с использованием k3s
2. **Docker образы** всех сервисов собраны и опубликованы в GitHub Container Registry
3. **Манифесты Kubernetes** созданы для всех компонентов: базы данных (StatefulSets), сервисы (Deployments), конфигурация (ConfigMap), секреты (Secrets), доступ извне (Ingress)
4. **Persistent storage** настроен для PostgreSQL, Redis и RabbitMQ через PersistentVolumeClaims
5. **Readiness/liveness пробы** настроены для всех сервисов для обеспечения отказоустойчивости

### Ключевые особенности реализации

| Компонент | Реализация |
|-----------|------------|
| **Базы данных** | StatefulSet с PVC (кроме Redis — Deployment с PVC) |
| **Сервисы** | Deployment с репликами (auth: 2, tasks: 2, остальные: 1) |
| **Конфигурация** | ConfigMap для несекретных параметров |
| **Секреты** | Secrets для паролей и URL подключений |
| **Доступ к реестру** | imagePullSecrets для доступа к приватным образам GHCR |
| **Маршрутизация** | Ingress с правилами на основе имён сервисов |
| **Балансировка** | Service (ClusterIP) + kube-proxy для распределения трафика |

### Преимущества подхода

1. **Декларативность** — всё описывается в YAML, версионируется вместе с кодом
2. **Масштабируемость** — увеличение реплик одной командой
3. **Отказоустойчивость** — при падении пода создаётся новый
4. **Единообразие** — одинаковые принципы для всех сервисов
5. **Изоляция** — namespace для проекта отделяет ресурсы

### Проблемы и их решения

| Проблема | Решение |
|----------|---------|
| RabbitMQ не проходил пробы | Увеличен timeoutSeconds до 5 |
| Поды не могли скачать образы | Добавлен imagePullSecrets в манифесты |
| Не хватало CPU для всех подов | Уменьшены requests.cpu до 50m |
| Ingress работал на нестандартном порту | Использован порт 30508 в URL |
| Health checks возвращали 404 | Добавлены хендлеры в код сервисов |

### Результаты

- Kubernetes кластер успешно развёрнут
- Все образы собраны и опубликованы
- Все манифесты применены
- Все поды в статусе Running
- Ingress работает и доступен извне
- API сервисов отвечает на запросы
- GraphQL playground доступен
- Масштабирование работает

Таким образом, проект полностью готов к эксплуатации в Kubernetes и может быть легко масштабирован при необходимости.

---

## Контрольные вопросы

### 1. Чем Pod отличается от Deployment?

**Pod** — это минимальная единица в Kubernetes, которая представляет собой один или несколько контейнеров, разделяющих сетевое пространство и storage. Pod может быть создан напрямую, но в этом случае он не будет автоматически восстанавливаться при сбоях.

**Deployment** — это ресурс более высокого уровня, который управляет ReplicaSet и обеспечивает:
- Декларативные обновления (rolling updates)
- Масштабирование (изменение количества реплик)
- Самовосстановление (при падении пода создаётся новый)
- Откаты к предыдущим версиям

В проекте используются Deployment'ы для всех сервисов, что обеспечивает отказоустойчивость и простоту масштабирования.

### 2. Зачем нужен Service и почему нельзя "ходить прямо в Pod"?

**Service** нужен потому что поды в Kubernetes:
1. **Эфемерны** — они могут быть пересозданы в любой момент и получить новый IP-адрес
2. **Масштабируются** — может быть несколько реплик, и клиент должен обращаться ко всем
3. **Балансируют нагрузку** — Service распределяет трафик между подами

Без Service пришлось бы:
- Отслеживать меняющиеся IP-адреса подов
- Вручную настраивать балансировку
- Обновлять конфигурацию при каждом перезапуске

В проекте Service используется для всех сервисов, обеспечивая стабильные имена (`auth-service`, `tasks-service` и т.д.) для внутренней коммуникации.

### 3. Чем readiness probe отличается от liveness probe?

**Readiness probe** определяет, готов ли под принимать трафик. Если probe не проходит, под исключается из балансировки (Endpoints удаляется из Service), но сам под не перезапускается. Используется для:
- Ожидания загрузки данных
- Проверки доступности зависимостей (БД, кэш)
- Прогрева кэша

**Liveness probe** определяет, жив ли процесс. Если probe не проходит, Kubernetes перезапускает под. Используется для обнаружения deadlock'ов, зависаний и других критических состояний.

В проекте:
- Для auth, tasks, graphql настроены HTTP пробы на `/health`
- Для rabbitmq настроена exec проба `rabbitmq-diagnostics ping`
- Readiness имеет меньший `initialDelaySeconds` (5-20), чем liveness (15-40)

### 4. Зачем нужен ConfigMap и чем он отличается от Secret?

**ConfigMap** предназначен для хранения несекретных конфигурационных данных в формате ключ-значение. Может содержать:
- Переменные окружения
- Файлы конфигурации
- Параметры запуска

**Secret** предназначен для хранения чувствительных данных: паролей, токенов, ключей. Отличается от ConfigMap:
- Данные хранятся в закодированном виде (base64)
- Может быть зашифрован на уровне etcd
- Более строгие права доступа
- Не попадает в логи и выводы команд (если не использовать --show-secrets)

В проекте:
- ConfigMap содержит порты, имена хостов, настройки TTL
- Secret содержит пароль БД и URL RabbitMQ

### 5. Почему важно использовать теги образов, а не только latest?

Использование только `latest` приводит к проблемам:

1. **Невоспроизводимость** — нельзя точно сказать, какая версия кода запущена
2. **Неожиданные изменения** — при перезапуске пода может подтянуться новая версия, что приведёт к неожиданному поведению
3. **Сложность отката** — невозможно быстро вернуться к предыдущей версии
4. **Проблемы с кэшированием** — Kubernetes может не обновить образ, если уже есть локальная копия

**Правильный подход:** использовать уникальные теги для каждой сборки (например, commit hash или версию) и отдельно обновлять `latest` для удобства. В проекте использованы теги `health-v1`, а в CI/CD можно использовать `${{ github.sha }}` как в работе №24.